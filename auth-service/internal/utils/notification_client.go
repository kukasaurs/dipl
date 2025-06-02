// internal/utils/notification_client.go
package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// NotificationPayload — структура, которую ждёт Notification Service.
type NotificationPayload struct {
	RecipientID   string                 `json:"recipient_id"`   // userID или любая сущность, кому шлём
	RecipientRole string                 `json:"recipient_role"` // "client", "cleaner", "manager", "admin" или "all"
	Title         string                 `json:"title"`
	Body          string                 `json:"body"`
	Type          string                 `json:"type"`    // из вашего ТЗ: "order_confirmed", "assigned_order" и т.д.
	Data          map[string]interface{} `json:"data"`    // любые доп. поля (например, order_id, service, время и т.п.)
	Channel       string                 `json:"channel"` // "email", "push", "sms"
}

// SendNotification отправляет запрос к Notification Service.
func SendNotification(ctx context.Context, payload NotificationPayload) error {
	// Адрес Notification Service (с учётом Docker Compose / environment)
	url := fmt.Sprintf("http://notification-service:8080/api/notifications/send")

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// HTTP-клиент с таймаутом
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("notification service returned status %d", resp.StatusCode)
	}
	return nil
}
