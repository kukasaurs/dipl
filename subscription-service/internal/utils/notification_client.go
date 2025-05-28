package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// NotificationServiceClient шлёт пуш/email/SMS через Notification-микросервис.
type NotificationServiceClient struct {
	URL string
}

func NewNotificationClient(url string) *NotificationServiceClient {
	return &NotificationServiceClient{URL: url}
}

// SendNotification отправляет message пользователю userID.
func (c *NotificationServiceClient) SendNotification(ctx context.Context, userID, message string) error {
	body := map[string]string{
		"user_id": userID,
		"message": message,
	}
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL+"/api/notifications/send", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("notification svc returned %d", resp.StatusCode)
	}
	return nil
}
