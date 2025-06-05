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
	Address        string          `json:"address"`
	ServiceType    string          `json:"service_type"`
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

func (c *OrderServiceClient) CreateOrderFromSubscription(ctx context.Context, sub models.Subscription, authHeader string) error {
	// 1) Сначала получаем оригинальный заказ, чтобы узнать address/service_ids и т.д.
	//    Из OrderResponse должно быть видно, по какому адресу и на какие услуги
	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL+"/orders/"+sub.OrderID.Hex(), nil)
	if err != nil {
		return fmt.Errorf("build GET order request: %w", err)
	}
	getReq.Header.Set("Authorization", authHeader)

	getResp, err := http.DefaultClient.Do(getReq)
	if err != nil {
		return fmt.Errorf("failed to GET original order: %w", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		return fmt.Errorf("order service returned %d on GET %s", getResp.StatusCode, c.URL+"/orders/"+sub.OrderID.Hex())
	}

	var original OrderResponse
	if err := json.NewDecoder(getResp.Body).Decode(&original); err != nil {
		return fmt.Errorf("decode original order response: %w", err)
	}

	if sub.NextPlannedDate == nil {
		return fmt.Errorf("subscription.NextPlannedDate is nil")
	}

	// 2) Формируем JSON для нового заказа на базе оригинального
	newBody := map[string]interface{}{
		"address":      original.Address,
		"service_type": original.ServiceType,
		"service_ids":  original.ServiceIDs,
		// Берём дату из подписки (NextPlannedDate) и сериализуем в RFC3339
		"date":    sub.NextPlannedDate.Format(time.RFC3339),
		"status":  "prepaid",                          // фиксируем prepaid
		"comment": "Автоматический заказ по подписке", // любой текст
	}
	jsonData, err := json.Marshal(newBody)
	if err != nil {
		return fmt.Errorf("marshal new order body: %w", err)
	}

	// 3) Отправляем POST /orders
	postReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL+"/orders", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("build POST order request: %w", err)
	}
	postReq.Header.Set("Content-Type", "application/json")
	postReq.Header.Set("Authorization", authHeader)

	postResp, err := http.DefaultClient.Do(postReq)
	if err != nil {
		return fmt.Errorf("failed to POST new order: %w", err)
	}
	defer postResp.Body.Close()

	if postResp.StatusCode >= 300 {
		return fmt.Errorf("order service returned %d on POST /orders", postResp.StatusCode)
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
