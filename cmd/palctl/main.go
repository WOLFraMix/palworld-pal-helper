package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/WOLFraMix/Palworld-pal-helper-withAliceAI-.git/internal/db"
	"github.com/WOLFraMix/Palworld-pal-helper-withAliceAI-.git/internal/importer"
	"github.com/WOLFraMix/Palworld-pal-helper-withAliceAI-.git/internal/overlay"
)

const dbPath = "data/palworld.db"
const dataDir = "data"

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}

	switch os.Args[1] {
	case "import":
		dir := dataDir
		if len(os.Args) > 2 {
			dir = os.Args[2]
		}
		store, err := db.Open(dbPath)
		if err != nil {
			fatal(err)
		}
		defer store.DB.Close()

		n, err := importer.ImportDir(store, dir)
		if err != nil {
			fatal(err)
		}
		fmt.Printf("импорт готов: обработано файлов %d, палов в базе %d\n", n, store.Count())

	case "search":
		q := strings.Join(os.Args[2:], " ")
		store, err := db.Open(dbPath)
		if err != nil {
			fatal(err)
		}
		defer store.DB.Close()

		pals, err := store.Search(q)
		if err != nil {
			fatal(err)
		}
		if len(pals) == 0 {
			fmt.Println("ничего не найдено")
			return
		}
		for _, p := range pals {
			fmt.Println(palLine(p))
			if p.SkillNameRu != "" {
				fmt.Println("  Навык: " + p.SkillNameRu)
			}
			if p.DescriptionRu != "" {
				fmt.Println("   " + p.DescriptionRu)
			}
		}

	case "watch":
		store, err := db.Open(dbPath)
		if err != nil {
			fatal(err)
		}
		defer store.DB.Close()

		fmt.Println("Оверлей запущен: Ctrl+Alt+Q — поиск по тексту из буфера обмена, клик по панели или 5 секунд — скрыть. Ctrl+C — выход.")
		err = overlay.Run(func(q string) string {
			q = strings.TrimSpace(q)
			q = strings.TrimPrefix(q, "пал ")
			found, err := store.Search(q)
			if err != nil || len(found) == 0 {
				return "Пал не найден: " + q
			}
			var b strings.Builder
			for i, p := range found {
				if i == 2 {
					fmt.Fprintf(&b, "… ещё %d, уточни запрос", len(found)-2)
					break
				}
				b.WriteString(palLine(p))
				b.WriteString("\n")
				if p.SkillNameRu != "" {
					b.WriteString("  Навык: ")
					b.WriteString(p.SkillNameRu)
					b.WriteString("\n")
				}
				if i == 0 && p.DescriptionRu != "" {
					b.WriteString("  ")
					b.WriteString(p.DescriptionRu)
				}
				if i < len(found)-1 {
					b.WriteString("\n")
				}
			}
			return strings.TrimRight(b.String(), "\n")
		})
		if err != nil {
			fmt.Println("ошибка оверлея:", err)
		}

	default:
		usage()
	}
}

func usage() {
	fmt.Println(`pal-helper — помощник по Palworld

Команды:
  palctl import [папка]   — импорт данных в базу (по умолчанию data/)
  palctl search <запрос>  — поиск пала по имени (рус. или англ.)
  palctl watch            — оверлей: Ctrl+Alt+Q ищет по тексту из буфера`)
}

func fatal(err error) {
	fmt.Println("ошибка:", err)
	os.Exit(1)
}

var elementRu = map[string]string{
	"Normal":      "нейтральный",
	"Fire":        "огонь",
	"Water":       "вода",
	"Electricity": "электричество",
	"Ice":         "лёд",
	"Dragon":      "дракон",
	"Dark":        "тьма",
	"Leaf":        "трава",
	"Earth":       "земля",
}

func elementRuStr(e string) string {
	var out []string
	for _, p := range strings.Split(e, ", ") {
		if ru, ok := elementRu[strings.TrimSpace(p)]; ok {
			out = append(out, ru)
		} else {
			out = append(out, p)
		}
	}
	return strings.Join(out, ", ")
}

func palLine(p db.Pal) string {
	s := ""
	if p.PalDexNo != "" {
		s = "[" + p.PalDexNo + "] "
	}
	s += p.NameRu
	if p.NameEn != "" {
		s += " / " + p.NameEn
	}
	if p.Element != "" {
		s += " (" + elementRuStr(p.Element) + ")"
	}
	return s
}
