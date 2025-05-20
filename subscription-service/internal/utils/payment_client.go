package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type PaymentServiceClient struct {
	URL string
}

func NewPaymentClient(url string) *PaymentServiceClient {
	return &PaymentServiceClient{URL: url}
}

func (p *PaymentServiceClient) PayForCleanings(ctx context.Context, clientID string, subscriptionID string, amount int) error {
	body := map[string]interface{}{
		"client_id":       clientID,
		"subscription_id": subscriptionID,
		"amount":          amount, // кол-во уборок
		"description":     fmt.Sprintf("Subscription payment: %d cleanings", amount),
	}

	data, _ := json.Marshal(body)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, p.URL+"/api/payments", bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode >= 300 {
		return fmt.Errorf("payment failed: %v", err)
	}
	return nil
}
