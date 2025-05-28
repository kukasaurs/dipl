// subscription-service/internal/utils/payment_client.go

package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// PaymentServiceClient умеет шлёпать запрос в мок-Payment Service на /payments
type PaymentServiceClient struct {
	URL string // например "http://payment-service:8005"
}

// NewPaymentClient создаёт клиента.
func NewPaymentClient(url string) *PaymentServiceClient {
	return &PaymentServiceClient{URL: url}
}

// ChargeSubscription шлёт POST /payments с {order_id, user_id, amount}
func (c *PaymentServiceClient) ChargeSubscription(ctx context.Context, orderID, userID string, amount int64) error {
	payload := struct {
		OrderID string `json:"order_id"`
		UserID  string `json:"user_id"`
		Amount  int64  `json:"amount"`
	}{
		OrderID: orderID,
		UserID:  userID,
		Amount:  amount,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payment payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL+"/payments", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("build payment request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("send payment request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("payment service returned status %d", resp.StatusCode)
	}
	return nil
}
