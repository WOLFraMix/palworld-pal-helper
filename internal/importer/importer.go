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
	rePalObj     = regexp.MustCompile(`[A-Za-z_$][\w$]*=\{id:"((?:[^"\\]|\\.)*)"`)
	reIndexField = regexp.MustCompile(`(?:^|[^A-Za-z_])index\s*:\s*"?(\d{1,3})"?`)
	reTypesField = regexp.MustCompile(`(?:^|[^A-Za-z_])(?:types|elements|elementType)\s*:\s*
$$
([^
$$
]*)\]`)
	reQuoted = regexp.MustCompile(`"((?:[^"\\]|\\.)*)"`)
)

// ImportDir читает все файлы данных из папки dir и заполняет базу.
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
		start, end int
		name       string
	}
	var entries []entry
	taken := map[int]bool{}

	// 1) редкий случай: имя стоит прямо перед partnerSkill
	for _, m := range reAdjacency.FindAllStringSubmatchIndex(src, -1) {
		brace := m[1] - 1
		_, end, ok := extractBlock(src, brace)
		if !ok {
			continue
		}
		entries = append(entries, entry{brace, end, unescape(src[m[2]:m[3]])})
		taken[brace] = true
	}
	// 2) все блоки partnerSkill
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

	// начала объектов палов вида mr={id:"flowerprince",...
	var objStarts []int
	for _, o := range rePalObj.FindAllStringIndex(src, -1) {
		objStarts = append(objStarts, o[0])
	}

	pals := make([]db.Pal, 0, len(entries))
	for i, e := range entries {
		prevEnd := 0
		if i > 0 {
			prevEnd = entries[i-1].end
		}
		// область полей текущего пала: строго от начала его объекта
		// до начала блока навыка — чужие дропы и умения не мешают
		before := src[prevEnd:e.start]
		for _, o := range objStarts {
			if o >= prevEnd && o < e.start {
				before = src[o:e.start]
			}
		}

		p := db.Pal{}
		p.SkillNameRu = fieldInBlock(src[e.start:e.end], "name")
		p.DescriptionRu = fieldInBlock(src[e.start:e.end], "desc")

		if e.name != "" {
			p.NameRu = e.name
		} else if v, ok := lastStringField(before, "name"); ok {
			p.NameRu = v
		}
		if p.NameRu == "" {
			continue
		}
		if v, ok := lastStringField(before, "key"); ok {
			p.NameEn = v
		}
		if v, ok := lastNumberField(before); ok {
			p.PalDexNo = v
		}
		if v := lastTypesField(before); v != "" {
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
					i++
				}
				i++
			}
		case '`':
			j := strings.IndexByte(src[i+1:], '`')
			if j < 0 {
				return "", i, false
			}
			i += j + 2
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

func lastNumberField(before string) (string, bool) {
	ms := reIndexField.FindAllStringSubmatch(before, -1)
	if len(ms) == 0 {
		return "", false
	}
	return ms[len(ms)-1][1], true
}

func lastTypesField(before string) string {
	ms := reTypesField.FindAllStringSubmatch(before, -1)
	if len(ms) == 0 {
		return ""
	}
	// единственная группа захвата — индекс 1 (раньше я ошибочно брала m[2])
	return joinQuoted(ms[len(ms)-1][1])
}

func joinQuoted(s string) string {
	var parts []string
	for _, q := range reQuoted.FindAllStringSubmatch(s, -1) {
		parts = append(parts, q[1])
	}
	return strings.Join(parts, ", ")
}

func unescape(s string) string {
	r := strings.NewReplacer(`\r`, " ", `\n`, " ", `\t`, " ", `\"`, `"`, `\\`, `\`)
	s = r.Replace(s)
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}
