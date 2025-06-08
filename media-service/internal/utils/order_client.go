package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// OrderResponse содержит только то, что нам нужно из Order Service
type OrderResponse struct {
	ID         string   `json:"id"`
	CleanerIDs []string `json:"cleaner_id"` // именно так приходит массив
}

// OrderServiceClient умеет дернуть Order Service
type OrderServiceClient struct {
	BaseURL string
	client  *http.Client
}

// NewOrderClient возвращает клиент с готовым http.Client
func NewOrderClient(baseURL string) *OrderServiceClient {
	return &OrderServiceClient{
		BaseURL: baseURL,
		client:  http.DefaultClient,
	}
}

// IsCleaner проверяет, что userID есть в cleaner_id заказа
func (oc *OrderServiceClient) IsCleaner(ctx context.Context, orderID, authHeader string) (bool, error) {
	url := fmt.Sprintf("%s/orders/%s", oc.BaseURL, orderID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", authHeader)

	resp, err := oc.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("call api-gateway: %w", err)
	}
	defer resp.Body.Close()

	// 200 OK — это «я назначенный клинер и могу посмотреть этот заказ»
	if resp.StatusCode == http.StatusOK {
		return true, nil
	}
	// любая другая — отказ
	return false, nil
}
func (c *OrderServiceClient) HasReports(orderID, authHeader string) (bool, error) {
	url := fmt.Sprintf("%s/media/reports/%s", c.BaseURL, orderID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", authHeader)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("media service returned %d", resp.StatusCode)
	}

	var medias []struct{ URL string }
	if err := json.NewDecoder(resp.Body).Decode(&medias); err != nil {
		return false, err
	}
	return len(medias) > 0, nil
}
