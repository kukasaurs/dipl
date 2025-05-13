package handler

import (
	"cleaning-app/payment-service/internal/utils"
	"encoding/json"
	"log"
	"net/http"
	"os"
)

type Handler struct {
	OrderServiceURL string
}

func NewHandler() *Handler {
	return &Handler{
		OrderServiceURL: os.Getenv("ORDER_SERVICE_URL"),
	}
}

func (h *Handler) Pay(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OrderID string `json:"order_id"`
		UserID  string `json:"user_id"`
		Amount  int64  `json:"amount"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	resp := map[string]string{
		"status":        "success",
		"client_secret": "mock_client_secret_123",
	}

	// 💬 отправляем уведомление в order-service
	go func() {
		if err := utils.NotifyOrderService(h.OrderServiceURL, req.OrderID, resp["status"]); err != nil {
			log.Printf("[NOTIFY ERROR] %v", err)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
