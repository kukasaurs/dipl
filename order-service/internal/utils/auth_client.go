package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type authClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewAuthClient(baseURL string) *authClient {
	return &authClient{
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

func (c *authClient) AddBulkRatings(ctx context.Context, cleanerIDs []string, rating int, comment string, authHeader string) error {
	payload := map[string]interface{}{
		"cleaner_ids": cleanerIDs,
		"rating":      rating,
		"comment":     comment,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/auth/add-ratings", c.baseURL), bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		respBody, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("auth service %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}
