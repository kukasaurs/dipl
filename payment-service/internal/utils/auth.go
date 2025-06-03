package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// GetBearerToken извлекает токен из заголовка Authorization: Bearer <token>.
// Если заголовка нет или он невалиден, возвращает пустую строку.
func GetBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) || len(auth) <= len(prefix) {
		return ""
	}
	return auth[len(prefix):]
}

// NewAuthenticatedRequest создаёт новый http.Request к url с методом method, телом body,
// и добавляет в него заголовок Authorization: Bearer <token>.
// Если token пустой, заголовок не устанавливается.
func NewAuthenticatedRequest(method, url, token string, body []byte) (*http.Request, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req, nil
}

// AttachBearerHeader добавляет токен в уже существующий http.Request.
// Если token пустой, ничего не делает.
func AttachBearerHeader(req *http.Request, token string) {
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

// NotifyJSONWithAuth отправляет POST с JSON-полем {"entity_id":..,"status":..}
// на указанный URL, добавляя Authorization: Bearer <token>.
// client должен быть уже инициализирован.
func NotifyJSONWithAuth(client *http.Client, url, entityID, status, token string) {
	payload := map[string]string{"entity_id": entityID, "status": status}
	b, _ := json.Marshal(payload)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(b))
	if err != nil {
		fmt.Printf("[utils] NotifyJSONWithAuth: create request failed: %v\n", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	AttachBearerHeader(req, token)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("[utils] NotifyJSONWithAuth: request error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("[utils] NotifyJSONWithAuth: unexpected status %d, body: %s\n", resp.StatusCode, string(body))
	}
}
