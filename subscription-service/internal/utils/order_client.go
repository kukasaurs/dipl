// subscription-service/internal/utils/order_client.go
package utils

import (
	"bytes"
	"cleaning-app/subscription-service/internal/models"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type ServiceDetail struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

type OrderResponse struct {
	ID             string          `json:"id"`
	ClientID       string          `json:"client_id"`
	ServiceIDs     []string        `json:"service_ids"`
	ServiceDetails []ServiceDetail `json:"service_details"`
	TotalPrice     float64         `json:"total_price"`
	CreatedAt      string          `json:"created_at"`
}

// OrderServiceClient умеет делать запросы к Order Service
type OrderServiceClient struct {
	URL string
}

func NewOrderClient(url string) *OrderServiceClient {
	return &OrderServiceClient{URL: url}
}

func (c *OrderServiceClient) CreateOrderFromSubscription(ctx context.Context, sub models.Subscription) error {
	body := map[string]interface{}{
		"order_id":        sub.OrderID.Hex(),
		"user_id":         sub.UserID.Hex(),
		"source":          "subscription",
		"subscription_id": sub.ID.Hex(),
		"date":            time.Now().Format("2006-01-02"),
	}

	jsonData, _ := json.Marshal(body)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.URL+"/orders", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode >= 300 {
		return fmt.Errorf("failed to create order from subscription")
	}
	return nil
}

// GetOrderByID проксирует заголовок Authorization в Order Service
func (c *OrderServiceClient) GetOrderByID(ctx context.Context, orderID, authHeader string) (*OrderResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL+"/orders/"+orderID, nil)
	if err != nil {
		return nil, fmt.Errorf("build get-order request: %w", err)
	}
	// Крайне важно: передаем тот же JWT, что пришел в Subscription Service
	req.Header.Set("Authorization", authHeader)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call order service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("order service returned status %d", resp.StatusCode)
	}

	var out OrderResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode order response: %w", err)
	}
	return &out, nil
}
