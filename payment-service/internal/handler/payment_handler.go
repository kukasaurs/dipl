package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"cleaning-app/payment-service/internal/utils"
)

type PaymentHandler struct {
	OrderServiceURL        string
	SubscriptionServiceURL string
	httpClient             *http.Client
}

func NewPaymentHandler(orderURL, subscriptionURL string) *PaymentHandler {
	return &PaymentHandler{
		OrderServiceURL:        orderURL,
		SubscriptionServiceURL: subscriptionURL,
		httpClient:             &http.Client{Timeout: 5 * time.Second},
	}
}

type PaymentRequest struct {
	EntityType string  `json:"entity_type"`
	EntityID   string  `json:"entity_id"`
	UserID     string  `json:"user_id"`
	Amount     float64 `json:"amount"`
}

type PaymentResponse struct {
	Status       string `json:"status"`
	ClientSecret string `json:"client_secret,omitempty"`
	Reason       string `json:"reason,omitempty"`
}

func (h *PaymentHandler) Pay(w http.ResponseWriter, r *http.Request) {
	log.Printf("[TRACE][PaymentService] Pay called: Method=%s, Path=%s", r.Method, r.URL.Path)

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// вытаскиваем JWT раз и навсегда
	token := utils.GetBearerToken(r)
	if token == "" {
		http.Error(w, "Authorization header missing or invalid", http.StatusUnauthorized)
		return
	}

	var reqBody PaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		log.Printf("[ERROR][PaymentService] invalid request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	log.Printf("[TRACE][PaymentService] Parsed PaymentRequest = %+v", reqBody)
	defer r.Body.Close()

	if reqBody.EntityType == "" || reqBody.EntityID == "" || reqBody.UserID == "" || reqBody.Amount <= 0 {
		http.Error(w, "entity_type, entity_id, user_id и amount обязательны и amount > 0", http.StatusBadRequest)
		return
	}

	switch reqBody.EntityType {
	case "order":
		ok, reason := h.validateOrder(reqBody.EntityID, reqBody.Amount, token)
		if !ok {
			resp := PaymentResponse{Status: "failed", Reason: reason}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(resp)
			return
		}

		go func(entityID, jwt string) {
			notifyURL := fmt.Sprintf("%s/api/internal/payments/notify", h.OrderServiceURL)
			utils.NotifyJSONWithAuth(h.httpClient, notifyURL, entityID, "success", jwt)
		}(reqBody.EntityID, token)

		resp := PaymentResponse{Status: "success", ClientSecret: "mock_client_secret_123"}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)

	case "subscription":
		log.Printf("[TRACE][PaymentService] validateSubscription for subID=%s amount=%.2f", reqBody.EntityID, reqBody.Amount)
		ok, reason := h.validateSubscription(reqBody.EntityID, reqBody.Amount, token)
		if !ok {
			log.Printf("[WARN][PaymentService] validateSubscription failed: %s", reason)
			resp := PaymentResponse{Status: "failed", Reason: reason}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(resp)
			return
		}

		go func(entityID, jwt string) {
			notifyURL := fmt.Sprintf("%s/api/payments/notify", h.SubscriptionServiceURL)
			utils.NotifyJSONWithAuth(h.httpClient, notifyURL, entityID, "success", jwt)
		}(reqBody.EntityID, token)

		resp := PaymentResponse{Status: "success", ClientSecret: "mock_client_secret_123"}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)

	default:
		log.Printf("[WARN][PaymentService] unknown entity_type: %s", reqBody.EntityType)
		http.Error(w, "entity_type должен быть 'order' или 'subscription'", http.StatusBadRequest)
	}
}

func (h *PaymentHandler) validateOrder(orderID string, amount float64, token string) (bool, string) {
	url := fmt.Sprintf("%s/orders/%s", h.OrderServiceURL, orderID)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	utils.AttachBearerHeader(req, token)
	res, err := h.httpClient.Do(req)
	if err != nil {
		return false, "Order Service unreachable"
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusNotFound:
		return false, "Order not found"
	case http.StatusUnauthorized:
		return false, "Order Service returned status 401"
	case http.StatusOK:
	default:
		return false, fmt.Sprintf("Order Service returned status %d", res.StatusCode)
	}

	var respBody struct {
		ID         string  `json:"id"`
		TotalPrice float64 `json:"total_price"`
	}
	if err := json.NewDecoder(res.Body).Decode(&respBody); err != nil {
		return false, "failed to parse Order Service response"
	}

	if respBody.TotalPrice != amount {
		return false, "amount mismatch"
	}
	return true, ""
}

func (h *PaymentHandler) validateSubscription(subID string, amount float64, token string) (bool, string) {
	url := fmt.Sprintf("%s/subscriptions/%s", h.SubscriptionServiceURL, subID)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	utils.AttachBearerHeader(req, token)
	res, err := h.httpClient.Do(req)
	if err != nil {
		return false, "Subscription Service unreachable"
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusNotFound:
		return false, "Subscription not found"
	case http.StatusUnauthorized:
		return false, "Subscription Service returned status 401"
	case http.StatusOK:
	default:
		return false, fmt.Sprintf("Subscription Service returned status %d", res.StatusCode)
	}

	var respBody struct {
		ID    string  `json:"id"`
		Price float64 `json:"price"`
	}
	if err := json.NewDecoder(res.Body).Decode(&respBody); err != nil {
		return false, "failed to parse Subscription Service response"
	}

	if respBody.Price != amount {
		return false, "amount mismatch"
	}
	return true, ""
}
