package main

import (
	"fmt"
	"log"
	"os"

	"github.com/WOLFraMix/Palworld-pal-helper-withAliceAI-.git/internal/api"
	"github.com/WOLFraMix/Palworld-pal-helper-withAliceAI-.git/internal/db"
	"github.com/WOLFraMix/Palworld-pal-helper-withAliceAI-.git/internal/importer"
)

const dbPath = "data/palworld.db"

func main() {
	if len(os.Args) < 2 {
		fmt.Println("использование: palctl import | search <имя> | serve")
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
			fmt.Printf("[%s] %s / %s\n  Навык: %s\n  %s\n",
				p.PalDexNo, p.NameRu, p.NameEn, p.SkillNameRu, p.DescriptionRu)
		}

	case "serve":
		if err := api.Run(store, "127.0.0.1:5277"); err != nil {
			log.Fatal(err)
		}

	default:
		fmt.Println("неизвестная команда")
	}
}
