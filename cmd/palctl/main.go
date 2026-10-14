package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/WOLFraMix/Palworld-pal-helper-withAliceAI-.git/internal/api"
	"github.com/WOLFraMix/Palworld-pal-helper-withAliceAI-.git/internal/bridge"
	"github.com/WOLFraMix/Palworld-pal-helper-withAliceAI-.git/internal/db"
	"github.com/WOLFraMix/Palworld-pal-helper-withAliceAI-.git/internal/importer"
)

const dbPath = "data/palworld.db"

func main() {
	if len(os.Args) < 2 {
		fmt.Println("использование: palctl import | search <имя> | serve | bridge <путь к папке мода>")
		os.Exit(1)
	}
	store, err := db.Open(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer store.DB.Close()

	switch os.Args[1] {
	case "import":
		n, err := importer.ImportDir(store, "data/raw")
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("импорт готов: обработано файлов %d, палов в базе %d\n", n, store.Count())

	case "search":
		if len(os.Args) < 3 {
			fmt.Println("укажи имя: palctl search ламболл")
			os.Exit(1)
		}
		pals, err := store.Search(os.Args[2])
		if err != nil {
			log.Fatal(err)
		}
		if len(pals) == 0 {
			fmt.Println("ничего не найдено")
			return
		}
		for _, p := range pals {
			fmt.Println(palLine(p))
			fmt.Println("  Навык:", p.SkillNameRu)
			fmt.Println("  ", p.DescriptionRu)
		}

	case "serve":
		if err := api.Run(store, "127.0.0.1:5277"); err != nil {
			log.Fatal(err)
		}

	case "bridge":
		if len(os.Args) < 3 {
			fmt.Println("укажи путь к папке мода:")
			fmt.Println(`  palctl bridge "C:\Games\Palworld\Pal\Binaries\Win64\ue4ss\Mods\PalHelper\bridge"`)
			os.Exit(1)
		}
		if err := bridge.Run(store, os.Args[2]); err != nil {
			log.Fatal(err)
		}

	default:
		fmt.Println("неизвестная команда")
	}
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
