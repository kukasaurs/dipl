package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type ManagerUser struct {
	ID   string `json:"id"`
	Role string `json:"role"`
	// можно добавить имя, email и т.п. при необходимости
}

func GetManagers(ctx context.Context, userServiceURL string) ([]ManagerUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userServiceURL+"/api/users/managers", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get managers: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("user-service returned status %d", resp.StatusCode)
	}

	var managers []ManagerUser
	if err := json.NewDecoder(resp.Body).Decode(&managers); err != nil {
		return nil, fmt.Errorf("failed to decode managers: %w", err)
	}

	return managers, nil
}
