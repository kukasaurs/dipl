package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type AuthClient struct {
	baseURL string
}

type AuthResponse struct {
	Valid bool   `json:"valid"`
	Role  string `json:"role"`
}

func NewAuthClient(baseURL string) *AuthClient {
	return &AuthClient{
		baseURL: baseURL,
	}
}

// JWTWithAuth middleware to verify user role against auth service
func JWTWithAuth(client *AuthClient, requiredRole string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check for Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Authorization header required", http.StatusUnauthorized)
				return
			}

			// Extract token
			token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
			if token == "" {
				http.Error(w, "Bearer token required", http.StatusUnauthorized)
				return
			}

			// Verify token with auth service
			valid, role, err := client.VerifyToken(token)
			if err != nil {
				http.Error(w, fmt.Sprintf("Auth verification failed: %v", err), http.StatusUnauthorized)
				return
			}

			if !valid {
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			// Check role
			if role != requiredRole {
				http.Error(w, "Insufficient privileges", http.StatusForbidden)
				return
			}

			// Continue with request
			next.ServeHTTP(w, r)
		})
	}
}

// VerifyToken verifies a token against the auth service
func (c *AuthClient) VerifyToken(token string) (bool, string, error) {
	url := fmt.Sprintf("%s/verify", c.baseURL)

	// Create request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, "", err
	}

	// Add token to header
	req.Header.Set("Authorization", "Bearer "+token)

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false, "", err
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return false, "", fmt.Errorf("auth service returned status: %d", resp.StatusCode)
	}

	// Parse response
	var authResp AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return false, "", err
	}

	return authResp.Valid, authResp.Role, nil
}
