package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/WOLFraMix/Palworld-pal-helper-withAliceAI-.git/internal/db"
)

func Run(store *db.Store, addr string) error {
	http.HandleFunc("/api/pal", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		pals, err := store.Search(r.URL.Query().Get("name"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if len(pals) == 0 {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
			return
		}
		json.NewEncoder(w).Encode(pals[0])
	})
	log.Printf("API слушает http://%s/api/pal?name=<имя пала>", addr)
	return http.ListenAndServe(addr, nil)
}
