package bridge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/WOLFraMix/Palworld-pal-helper-withAliceAI-.git/internal/db"
)

// Run слушает папку in/, отвечает файлами в out/.
// Тот же путь, что в GAME_DIR/.../bridge у Lua-мода.
func Run(store *db.Store, dir string) error {
	in := filepath.Join(dir, "in")
	out := filepath.Join(dir, "out")
	for _, d := range []string{in, out} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	fmt.Printf("мост слушает: %s\n", dir)

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
				answer := handle(store, strings.TrimSpace(string(data)))

				// атомарная запись, чтобы Lua не поймал пол-файла
				tmp := filepath.Join(out, name+".tmp")
				if err := os.WriteFile(tmp, []byte(answer), 0o644); err == nil {
					os.Rename(tmp, filepath.Join(out, name))
				}
				os.Remove(inPath)
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
}

func handle(store *db.Store, query string) string {
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
