package importer

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/WOLFraMix/Palworld-pal-helper-withAliceAI-.git/internal/db"
)

var (
	reSingleFile = regexp.MustCompile(`^\d+_`)
	reAdjacency  = regexp.MustCompile(`name\s*:\s*"((?:[^"\\]|\\.)*)"\s*,\s*partnerSkill\s*:\s*\{`)
	reSkillKey   = regexp.MustCompile(`partnerSkill\s*:\s*\{`)
	reNumField   = regexp.MustCompile(`(?:^|[^A-Za-z_])(?:number|palNo|paldeckNo|no)\s*:\s*"?(\d{1,3})"?`)
	reTypesField = regexp.MustCompile(`(?:^|[^A-Za-z_])(?:types|elements|elementType)\s*:\s*
$$
([^
$$
]*)\]`)
	reQuoted = regexp.MustCompile(`"([^"]*)"`)
)

// ImportDir читает все файлы данных из папки dir и заполняет базу.
// Файлы вида "001_Ламболл.txt" — одиночные payload-ответы,
// остальные (.js) — большой бандл со всеми палами.
func ImportDir(store *db.Store, dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("не могу прочитать папку %s: %w", dir, err)
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext != ".txt" && ext != ".json" && ext != ".js" {
			continue
		}
		path := filepath.Join(dir, e.Name())

		var pals []db.Pal
		if reSingleFile.MatchString(e.Name()) {
			p, err := ParsePayloadFile(path)
			if err != nil {
				fmt.Printf("  пропуск %s: %v\n", e.Name(), err)
				continue
			}
			pals = []db.Pal{p}
		} else {
			pals, err = ParseBundle(path)
			if err != nil {
				fmt.Printf("  пропуск %s: %v\n", e.Name(), err)
				continue
			}
		}

		saved := 0
		for _, p := range pals {
			if p.SkillNameRu == "" {
				continue
			}
			if err := store.Upsert(p); err != nil {
				return n, err
			}
			saved++
		}
		fmt.Printf("  %s: добавлено/обновлено палов — %d\n", e.Name(), saved)
		n += saved
	}
	return n, nil
}

// ParsePayloadFile разбирает один сохранённый ответ _payload.json.
// Имя файла задаёт пала: "001_Ламболл.txt" → №001, "Ламболл";
// "001_Ламболл_Lamball.txt" — плюс английское имя.
func ParsePayloadFile(path string) (db.Pal, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return db.Pal{}, err
	}
	src := string(raw)

	pal := db.Pal{}
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	parts := strings.SplitN(base, "_", 3)
	switch {
	case len(parts) >= 3:
		pal.PalDexNo, pal.NameRu, pal.NameEn = parts[0], parts[1], parts[2]
	case len(parts) == 2:
		pal.PalDexNo, pal.NameRu = parts[0], parts[1]
	default:
		return pal, fmt.Errorf("имя файла должно быть вида 001_Ламболл.txt")
	}

	loc := reSkillKey.FindStringIndex(src)
	if loc == nil {
		return pal, fmt.Errorf("не нашёл блок partnerSkill")
	}
	block, _, ok := extractBlock(src, loc[1]-1)
	if !ok {
		return pal, fmt.Errorf("блок partnerSkill обрывается — файл сохранён не полностью")
	}
	pal.SkillNameRu = fieldInBlock(block, "name")
	pal.DescriptionRu = fieldInBlock(block, "desc")
	return pal, nil
}

// ParseBundle разбирает большой JS-файл (типа CD0oDVDf.js), где лежат все палы.
func ParseBundle(path string) ([]db.Pal, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	src := string(raw)

	type entry struct {
		start, end int // границы блока partnerSkill
		name       string
	}
	var entries []entry
	taken := map[int]bool{}

	// 1) надёжный случай: имя стоит прямо перед partnerSkill
	for _, m := range reAdjacency.FindAllStringSubmatchIndex(src, -1) {
		brace := m[1] - 1 // позиция "{"
		_, end, ok := extractBlock(src, brace)
		if !ok {
			continue
		}
		entries = append(entries, entry{brace, end, unescape(src[m[2]:m[3]])})
		taken[brace] = true
	}
	// 2) остальные блоки: имя ищем ближайшим "name:" перед блоком
	for _, loc := range reSkillKey.FindAllStringIndex(src, -1) {
		brace := loc[1] - 1
		if taken[brace] {
			continue
		}
		_, end, ok := extractBlock(src, brace)
		if !ok {
			continue
		}
		entries = append(entries, entry{brace, end, ""})
		taken[brace] = true
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("не найдено ни одного блока partnerSkill")
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].start < entries[j].start })

	pals := make([]db.Pal, 0, len(entries))
	for i, e := range entries {
		p := db.Pal{}
		p.SkillNameRu = fieldInBlock(src[e.start:e.end], "name")
		p.DescriptionRu = fieldInBlock(src[e.start:e.end], "desc")

		prevEnd := 0
		if i > 0 {
			prevEnd = entries[i-1].end
		}
		nextStart := len(src)
		if i < len(entries)-1 {
			nextStart = entries[i+1].start
		}
		before := src[prevEnd:e.start]
		after := src[e.end:nextStart]

		if e.name != "" {
			p.NameRu = e.name
		} else if v, ok := lastStringField(before, "name"); ok {
			p.NameRu = v
		}
		if p.NameRu == "" {
			continue // без имени не опознаём
		}
		if v, ok := firstNumberField(before, after); ok {
			p.PalDexNo = v
		}
		if v := firstTypesField(before, after); v != "" {
			p.Element = v
		}
		pals = append(pals, p)
	}
	return pals, nil
}

// extractBlock возвращает текст блока от "{" до парной "}",
// не спотыкаясь о скобки внутри строк "..." и `...`.
func extractBlock(src string, brace int) (block string, end int, ok bool) {
	if brace >= len(src) || src[brace] != '{' {
		return "", brace, false
	}
	depth := 0
	for i := brace; i < len(src); i++ {
		switch src[i] {
		case '"':
			i++
			for i < len(src) && src[i] != '"' {
				if src[i] == '\\' {
					i++ // экранированный символ
				}
				i++
			}
		case '`':
			j := strings.IndexByte(src[i+1:], '`')
			if j < 0 {
				return "", i, false // незакрытый бэктик — файл обрезан
			}
			i += j + 2 // перепрыгиваем всё содержимое
			continue
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return src[brace : i+1], i + 1, true
			}
		}
	}
	return "", len(src), false
}

// fieldInBlock достаёт значение поля key из блока — в кавычках или бэктиках.
func fieldInBlock(block, key string) string {
	if v, ok := lastStringField(block, key); ok {
		return v
	}
	re := regexp.MustCompile("(?s)(?:^|[^A-Za-z_])" + key + `\s*:\s*` + "`" + `([^` + "`" + `]*)`)
	if m := re.FindStringSubmatch(block); m != nil {
		return unescape(m[1])
	}
	return ""
}

func lastStringField(src, key string) (string, bool) {
	re := regexp.MustCompile(`(?:^|[^A-Za-z_])` + key + `\s*:\s*"((?:[^"\\]|\\.)*)"`)
	ms := re.FindAllStringSubmatch(src, -1)
	if len(ms) == 0 {
		return "", false
	}
	return unescape(ms[len(ms)-1][1]), true
}

func firstNumberField(before, after string) (string, bool) {
	if m := reNumField.FindStringSubmatch(before); m != nil {
		return m[1], true
	}
	if m := reNumField.FindStringSubmatch(after); m != nil {
		return m[1], true
	}
	return "", false
}

func firstTypesField(before, after string) string {
	m := reTypesField.FindStringSubmatch(before)
	if m == nil {
		m = reTypesField.FindStringSubmatch(after)
		if m == nil {
			return ""
		}
	}
	var parts []string
	for _, s := range reQuoted.FindAllStringSubmatch(m[2], -1) {
		parts = append(parts, s[1])
	}
	return strings.Join(parts, ", ")
}

func unescape(s string) string {
	r := strings.NewReplacer(`\r`, " ", `\n`, " ", `\t`, " ", `\"`, `"`, `\\`, `\`)
	s = r.Replace(s)
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}
