package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type PaymentNotification struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}

func NotifyOrderService(orderServiceURL, orderID, status string) error {
	payload := PaymentNotification{OrderID: orderID, Status: status}
	data, _ := json.Marshal(payload)

	url := orderServiceURL + "/api/internal/payments/notify"
	log.Printf("[NOTIFY] sending to %s: %+v", url, payload)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to notify order service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected response from order service: %d", resp.StatusCode)
	}
	log.Printf("[NOTIFY] success: %s", orderID)
	return nil
}
