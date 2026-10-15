package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/WOLFraMix/Palworld-pal-helper-withAliceAI-.git/internal/db"
)

// Run поднимает локальный веб-сервер: / — страница поиска, /api/pal — JSON.
func Run(store *db.Store, addr string) error {
	http.HandleFunc("/api/pal", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		pals, err := store.Search(r.URL.Query().Get("name"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if len(pals) == 0 {
			json.NewEncoder(w).Encode([]db.Pal{})
			return
		}
		json.NewEncoder(w).Encode(pals)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, pageHTML)
	})

	log.Printf("Палдекс: http://%s", addr)
	return http.ListenAndServe(addr, nil)
}

const pageHTML = `<!DOCTYPE html>
<html lang="ru">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Палхелпер — Палдекс</title>
<style>
  :root { color-scheme: dark; }
  body {
    margin: 0; min-height: 100vh;
    background: #16181d; color: #e8e4d8;
    font-family: "Segoe UI", sans-serif;
    display: flex; flex-direction: column; align-items: center;
  }
  h1 { margin: 28px 0 4px; font-weight: 600; letter-spacing: .5px; }
  .sub { color: #8a8f9c; margin-bottom: 22px; font-size: 14px; }
  #q {
    width: min(560px, 88vw); padding: 12px 18px; font-size: 17px;
    background: #23262e; color: #e8e4d8;
    border: 1px solid #3a3f4c; border-radius: 10px; outline: none;
  }
  #q:focus { border-color: #7aa2c8; }
  #results { width: min(560px, 88vw); margin: 18px 0 40px; }
  .card {
    background: #1e2129; border: 1px solid #2e323d;
    border-radius: 12px; padding: 14px 18px; margin-bottom: 12px;
  }
  .card h2 { margin: 0 0 6px; font-size: 17px; font-weight: 600; }
  .card .en { color: #8a8f9c; font-weight: 400; font-size: 14px; }
  .tags { margin: 6px 0; }
  .tag {
    display: inline-block; padding: 2px 10px; margin-right: 6px;
    border-radius: 20px; font-size: 12.5px; color: #fff;
  }
  .skill { color: #c8b273; margin: 6px 0 2px; }
  .desc { color: #b9bdc9; line-height: 1.45; font-size: 14px; }
</style>
</head>
<body>
<h1>Палдекс</h1>
<div class="sub">поиск по Palworld — имя на русском или английском</div>
<input id="q" placeholder="Например: Дандилорд, Lamball, камоноске игнис…" autofocus>
<div id="results"></div>
<script>
const q = document.getElementById("q");
const box = document.getElementById("results");
let timer = null;

q.addEventListener("input", () => {
  clearTimeout(timer);
  timer = setTimeout(search, 250);
});

async function search() {
  const s = q.value.trim();
  if (!s) { box.innerHTML = ""; return; }
  const r = await fetch("/api/pal?name=" + encodeURIComponent(s));
  const pals = await r.json();
  if (!pals.length) {
    box.innerHTML = '<div class="card">Не нашла пала: ' + esc(s) + "</div>";
    return;
  }
  box.innerHTML = pals.map(render).join("");
}

function esc(s) {
  return s.replace(/[&<>"']/g, c => ({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"}[c]));
}

const elRu = {
  "Normal": "нейтральный", "Fire": "огонь", "Water": "вода",
  "Electricity": "электричество", "Ice": "лёд", "Dragon": "дракон",
  "Dark": "тьма", "Leaf": "трава", "Earth": "земля"
};

const elColors = {
  "нейтральный": "#9a9a9a", "огонь": "#e06a3a", "вода": "#4a90d9",
  "электричество": "#e8c93a", "лёд": "#7ec8e3", "дракон": "#8a5ad9",
  "тьма": "#5a4a6e", "трава": "#5aa84a", "земля": "#b0813f"
};

function render(p) {
  const tags = (p.Element || "").split(",").map(s => s.trim()).filter(Boolean).map(e => {
    const ru = elRu[e] || e;
    return '<span class="tag" style="background:' + (elColors[ru] || "#666") + '">' + esc(ru) + "</span>";
  }).join("");
  let h = '<div class="card"><h2>' + (p.PalDexNo ? "[" + esc(p.PalDexNo) + "] " : "")
       + esc(p.NameRu) + ' <span class="en">/ ' + esc(p.NameEn) + "</span></h2>"
       + '<div class="tags">' + tags + "</div>";
  if (p.SkillNameRu) h += '<div class="skill">Навык: ' + esc(p.SkillNameRu) + "</div>";
  if (p.DescriptionRu) h += '<div class="desc">' + esc(p.DescriptionRu) + "</div>";
  return h + "</div>";
}
</script>
</body>
</html>`
