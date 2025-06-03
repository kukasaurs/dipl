package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type PaymentRequest struct {
	EntityType string  `json:"entity_type"`
	EntityID   string  `json:"entity_id"`
	UserID     string  `json:"user_id"`
	Amount     float64 `json:"amount"`
}

type PaymentServiceClient struct {
	URL string
}

func NewPaymentClient(url string) *PaymentServiceClient {
	return &PaymentServiceClient{URL: url}
}

func (c *PaymentServiceClient) Charge(ctx context.Context, entityType string, entityID string, userID string, authHeader string, amount float64) error {
	body := PaymentRequest{
		EntityType: entityType,
		EntityID:   entityID,
		UserID:     userID,
		Amount:     amount,
	}
	payload, err := json.Marshal(body)
	if err != nil {

		return fmt.Errorf("marshal payment request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL+"/payments", bytes.NewBuffer(payload))
	if err != nil {

		return fmt.Errorf("build payment request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {

		return fmt.Errorf("call payment service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {

		return fmt.Errorf("payment service returned status %d", resp.StatusCode)
	}

	return nil
}
