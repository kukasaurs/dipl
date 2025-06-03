package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type NotifyPayload struct {
	EntityID string `json:"entity_id"`
	Status   string `json:"status"`
}

func NotifyURL(client *http.Client, url, entityID, status string) {
	payload := NotifyPayload{EntityID: entityID, Status: status}
	b, _ := json.Marshal(payload)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(b))
	if err != nil {
		fmt.Printf("[NotifyURL] Failed to create request: %v\n", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("[NotifyURL] Request error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		fmt.Printf("[NotifyURL] Unexpected status %d, body: %s\n", resp.StatusCode, string(bodyBytes))
	}
}
