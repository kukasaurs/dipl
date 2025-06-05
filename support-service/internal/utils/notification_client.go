package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type NotificationClient struct {
	URL string
}

func NewNotificationClient(url string) *NotificationClient {
	return &NotificationClient{URL: url}
}

func (n *NotificationClient) SendMessageNotification(ctx context.Context, toUserID, messageText string) error {
	payload := map[string]interface{}{
		"user_id": toUserID,
		"type":    "support_message",
		"data": map[string]string{
			"text": messageText,
		},
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, n.URL+"/api/notifications/send", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode >= 300 {
		return fmt.Errorf("failed to send notification")
	}
	return nil
}
