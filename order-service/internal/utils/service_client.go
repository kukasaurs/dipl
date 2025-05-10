package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"cleaning-app/order-service/internal/models" // убедись, что путь корректный
)

type ServiceRequest struct {
	IDs []string `json:"ids"`
}

func FetchServiceDetails(ctx context.Context, baseURL string, serviceIDs []string) ([]models.Service, error) {
	requestBody, err := json.Marshal(ServiceRequest{IDs: serviceIDs})
	if err != nil {
		return nil, fmt.Errorf("marshal error: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/services/by-ids", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("new request error: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("non-200 from service: %d", resp.StatusCode)
	}

	var services []models.Service
	if err := json.NewDecoder(resp.Body).Decode(&services); err != nil {
		return nil, fmt.Errorf("decode error: %w", err)
	}

	return services, nil
}
