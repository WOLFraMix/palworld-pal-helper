package bridge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/WOLFraMix/Palworld-pal-helper-withAliceAI-.git/internal/api"
	"github.com/WOLFraMix/Palworld-pal-helper-withAliceAI-.git/internal/db"
)

// Run слушает папку in/ — запросы пишет туда Lua-мод на сервере.
// Ответ: файл в out/ (для отладки) + сообщение в игровой чат через REST API.
func Run(store *db.Store, dir string) error {
	in := filepath.Join(dir, "in")
	out := filepath.Join(dir, "out")
	for _, d := range []string{in, out} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	fmt.Printf("бот слушает запросы: %s\n", dir)

	srvAddr := os.Getenv("PAL_SERVER")
	if srvAddr == "" {
		srvAddr = "127.0.0.1:8212"
	}
	srvPass := os.Getenv("PAL_PASSWORD")

	for {
		entries, err := os.ReadDir(in)
		if err == nil {
			for _, e := range entries {
				name := e.Name()
				if e.IsDir() || !strings.HasSuffix(name, ".txt") {
					continue
				}
				inPath := filepath.Join(in, name)
				data, err := os.ReadFile(inPath)
				if err != nil {
					continue // файл ещё дописывается, возьмём на следующем круге
				}
				q := strings.TrimSpace(string(data))
				answer := handle(store, q)

				// атомарная запись ответа — для отладки
				tmp := filepath.Join(out, name+".tmp")
				if err := os.WriteFile(tmp, []byte(answer), 0o644); err == nil {
					os.Rename(tmp, filepath.Join(out, name))
				}
				os.Remove(inPath)

				// ответ в игровой чат
				fmt.Printf("запрос: %q -> %q\n", q, answer)
				if err := api.Announce(srvAddr, srvPass, answer); err != nil {
					fmt.Println("в чат не ушло:", err)
				}
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
}

func handle(store *db.Store, query string) string {
	nq := strings.ToLower(strings.Join(strings.Fields(query), " "))
	switch nq {
	case "кто онлайн", "ктоонлайн", "online", "кто тут":
		return onlineList()
	}
	if query == "" {
		return "пустой запрос — напиши в чат: pal <имя>"
	}
	pals, err := store.Search(query)
	if err != nil {
		return "ошибка поиска: " + err.Error()
	}
	if len(pals) == 0 {
		return "не нашла пала: " + query
	}
	p := pals[0]
	no := p.PalDexNo
	if no == "" {
		no = "??"
	}
	return fmt.Sprintf("[%s] %s — %s: %s", no, p.NameRu, p.SkillNameRu, p.DescriptionRu)
}

func onlineList() string {
	addr := os.Getenv("PAL_SERVER")
	if addr == "" {
		addr = "127.0.0.1:8212"
	}
	players, err := api.Players(addr, os.Getenv("PAL_PASSWORD"))
	if err != nil {
		return "не могу узнать онлайн: " + err.Error()
	}
	if len(players) == 0 {
		return "на сервере сейчас никого нет"
	}
	var parts []string
	for _, p := range players {
		parts = append(parts, fmt.Sprintf("%s (ур. %d, пинг %d мс)", p.Name, p.Level, int(p.Ping)))
	}
	return fmt.Sprintf("На сервере %d: %s", len(players), strings.Join(parts, ", "))
}
