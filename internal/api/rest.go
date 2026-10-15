package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Announce отправляет сообщение в чат выделенного сервера Palworld
// через официальный REST API (/v1/api/announce).
func Announce(addr, password, msg string) error {
	if password == "" {
		return fmt.Errorf("не задан пароль администратора (переменная PAL_PASSWORD)")
	}
	if r := []rune(msg); len(r) > 400 {
		msg = string(r[:400]) + "…"
	}
	body, err := json.Marshal(map[string]string{"message": msg})
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", "http://"+addr+"/v1/api/announce", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("admin", password)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("сервер недоступен на %s: %w", addr, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("сервер ответил %s (проверь RESTAPIEnabled и пароль)", resp.Status)
	}
	return nil
}

// Player — запись из ответа /v1/api/players.
type Player struct {
	Name     string  `json:"name"`
	UserID   string  `json:"userId"`
	Ping     float64 `json:"ping"`
	Level    int     `json:"level"`
	Location struct {
		X float64 `json:"location_x"`
		Y float64 `json:"location_y"`
	} `json:"-"`
}

// Players возвращает список игроков на сервере.
func Players(addr, password string) ([]Player, error) {
	if password == "" {
		return nil, fmt.Errorf("не задан пароль администратора (переменная PAL_PASSWORD)")
	}
	req, err := http.NewRequest("GET", "http://"+addr+"/v1/api/players", nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth("admin", password)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("сервер недоступен на %s: %w", addr, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("сервер ответил %s (проверь RESTAPIEnabled и пароль)", resp.Status)
	}
	var out struct {
		Players []Player `json:"players"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Players, nil
}
