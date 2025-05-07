package utils

import (
	"bytes"
	"cleaning-app/auth-service/internal/config"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type NotificationRequest struct {
	UserID       string            `json:"user_id"`
	Role         string            `json:"role"`
	Title        string            `json:"title"`
	Message      string            `json:"message"`
	Type         string            `json:"type"`
	DeliveryType string            `json:"delivery_type"` // push | email | both
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// SendNotification отправляет уведомление в сервис уведомлений
func SendNotification(ctx context.Context, cfg *config.Config, notification NotificationRequest) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	jsonData, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.NotifiServiceURL+"/api/notifications/send", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create notification request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("notification service returned status %d", resp.StatusCode)
	}
	return nil
}
