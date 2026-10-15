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
