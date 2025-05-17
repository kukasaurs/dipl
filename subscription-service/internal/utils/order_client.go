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

type OrderServiceClient struct {
	URL string
}

func NewOrderClient(url string) *OrderServiceClient {
	return &OrderServiceClient{URL: url}
}

func (c *OrderServiceClient) CreateOrderFromSubscription(ctx context.Context, sub models.Subscription) error {
	body := map[string]interface{}{
		"client_id":       sub.ClientID,
		"service_ids":     sub.ServiceIDs,
		"source":          "subscription",
		"subscription_id": sub.ID.Hex(),
		"date":            time.Now().Format("2006-01-02"),
	}
	jsonData, _ := json.Marshal(body)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.URL+"/api/orders", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode >= 300 {
		return fmt.Errorf("failed to create order from subscription")
	}
	return nil
}
