package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/WOLFraMix/Palworld-pal-helper-withAliceAI-.git/internal/api"
	"github.com/WOLFraMix/Palworld-pal-helper-withAliceAI-.git/internal/bridge"
	"github.com/WOLFraMix/Palworld-pal-helper-withAliceAI-.git/internal/db"
	"github.com/WOLFraMix/Palworld-pal-helper-withAliceAI-.git/internal/importer"
	"github.com/WOLFraMix/Palworld-pal-helper-withAliceAI-.git/internal/overlay"
)

const dbPath = "data/palworld.db"
const dataDir = `C:\dev\pal-helper\data\raw`

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}

	switch os.Args[1] {
	case "import":
		dir := dataDir
		fresh := false
		var rest []string
		for _, a := range os.Args[2:] {
			if a == "--fresh" {
				fresh = true
			} else {
				rest = append(rest, a)
			}
		}
		if len(rest) > 0 {
			dir = rest[0]
		}
		store, err := db.Open(dbPath)
		if err != nil {
			fatal(err)
		}
		defer store.DB.Close()

		if fresh {
			if err := store.Clear(); err != nil {
				fatal(err)
			}
			fmt.Println("старые данные удалены")
		}
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
		srvAddr := os.Getenv("PAL_SERVER")
		srvPass := os.Getenv("PAL_PASSWORD")
		if srvAddr != "" {
			fmt.Println("Режим чата: ответы будут уходить на сервер " + srvAddr)
		}
		err = overlay.Run(func(q string) string {
			q = strings.TrimSpace(q)
			q = strings.TrimPrefix(q, "пал ")
			found, err := store.Search(q)
			if err != nil || len(found) == 0 {
				return "Пал не найден: " + q
			}
			// точное совпадение имени — всегда первым
			nq := normalizeName(q)
			for i, p := range found {
				if normalizeName(p.NameRu) == nq || normalizeName(p.NameEn) == nq {
					if i > 0 {
						found[0], found[i] = found[i], found[0]
					}
					break
				}
			}
			p := found[0]
			var b strings.Builder
			b.WriteString(palLine(p))
			if p.SkillNameRu != "" {
				b.WriteString("\n  Навык: ")
				b.WriteString(p.SkillNameRu)
			}
			if p.DescriptionRu != "" {
				b.WriteString("\n  ")
				b.WriteString(p.DescriptionRu)
			}
			tx := b.String()
			if srvAddr != "" {
				if err := api.Announce(srvAddr, srvPass, tx); err != nil {
					fmt.Println("в чат не ушло:", err)
				}
			}
			return tx
		})
		if err != nil {
			fmt.Println("ошибка оверлея:", err)
		}

	case "serve":
		addr := "localhost:8080"
		if len(os.Args) > 2 {
			addr = os.Args[2]
		}
		store, err := db.Open(dbPath)
		if err != nil {
			fatal(err)
		}
		defer store.DB.Close()

		err = api.Run(store, addr)
		if err != nil {
			fatal(err)
		}

	case "say":
		msg := strings.Join(os.Args[2:], " ")
		if msg == "" {
			fmt.Println("использование: palctl say <сообщение>")
			return
		}
		addr := os.Getenv("PAL_SERVER")
		if addr == "" {
			addr = "127.0.0.1:8212"
		}
		if err := api.Announce(addr, os.Getenv("PAL_PASSWORD"), msg); err != nil {
			fatal(err)
		}
		fmt.Println("отправлено:", msg)

	case "bot":
		dir := `C:\dev\pal-helper\bridge`
		if len(os.Args) > 2 {
			dir = os.Args[2]
		}
		store, err := db.Open(dbPath)
		if err != nil {
			fatal(err)
		}
		defer store.DB.Close()

		err = bridge.Run(store, dir)
		if err != nil {
			fatal(err)
		}

	case "online":
		addr := os.Getenv("PAL_SERVER")
		if addr == "" {
			addr = "127.0.0.1:8212"
		}
		players, err := api.Players(addr, os.Getenv("PAL_PASSWORD"))
		if err != nil {
			fatal(err)
		}
		if len(players) == 0 {
			fmt.Println("на сервере сейчас никого нет")
			return
		}
		for _, p := range players {
			fmt.Printf("%s — ур. %d, пинг %d мс, локация (%.0f, %.0f)\n",
				p.Name, p.Level, int(p.Ping), p.Location.X, p.Location.Y)
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
  palctl watch            — оверлей: Ctrl+Alt+Q ищет по тексту из буфера
  palctl serve [адрес]    — Палдекс в браузере (по умолчанию localhost:8080)
  palctl say <сообщение>  — отправить сообщение в чат сервера (PAL_SERVER, PAL_PASSWORD)
  palctl bot [папка]      — чат-бот сервера (мост PalServer ↔ база)
  palctl online           — кто сейчас на сервере (PAL_SERVER, PAL_PASSWORD)
  palctl import [--fresh] [папка] — импорт данных (--fresh: пересобрать с нуля)`)
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

func normalizeName(s string) string {
	s = strings.ToLower(s)
	s = strings.NewReplacer("-", " ", "_", " ").Replace(s)
	return strings.Join(strings.Fields(s), " ")
}
