package importer

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/WOLFraMix/Palworld-pal-helper-withAliceAI-.git/internal/db"
)

// Блок навыка в payload-файле выглядит как partnerSkill:{...}
var skillBlockRe = regexp.MustCompile(`partnerSkill:\{[\s\S]*?\}`)

// Пары "поле: "значение"" внутри блока (ключи в payload не в кавычках).
var strFieldRe = regexp.MustCompile(`(\w+):\s*"((?:[^"\\]|\\.)*)"`)

// ImportDir читает все .txt/.json файлы из папки и заполняет базу.
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
		if ext != ".txt" && ext != ".json" {
			continue
		}
		pal, err := ParsePayloadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			fmt.Printf("  пропуск %s: %v\n", e.Name(), err)
			continue
		}
		if pal.SkillNameRu == "" {
			fmt.Printf("  пропуск %s: не нашёл навык в payload\n", e.Name())
			continue
		}
		if err := store.Upsert(pal); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// ParsePayloadFile разбирает сохранённый ответ _payload.json со страницы пала.
// Имя файла задаёт пала: "001_Ламболл.txt" → №001, имя "Ламболл";
// "001_Ламболл_Lamball.txt" — дополнительно английское имя.
func ParsePayloadFile(path string) (db.Pal, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return db.Pal{}, err
	}
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

	block := skillBlockRe.FindString(string(raw))
	if block == "" {
		return pal, fmt.Errorf("не нашёл блок partnerSkill")
	}
	for _, f := range strFieldRe.FindAllStringSubmatch(block, -1) {
		switch f[1] {
		case "name":
			if pal.SkillNameRu == "" {
				pal.SkillNameRu = unescape(f[2])
			}
		case "desc":
			if pal.DescriptionRu == "" {
				pal.DescriptionRu = unescape(f[2])
			}
		}
	}
	return pal, nil
}

func unescape(s string) string {
	r := strings.NewReplacer(`\r`, " ", `\n`, " ", `\"`, `"`, `\\`, `\`)
	return strings.TrimSpace(r.Replace(s))
}
