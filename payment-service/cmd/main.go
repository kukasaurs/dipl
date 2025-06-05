package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"cleaning-app/payment-service/internal/handler"
)

func main() {
	// Считываем URL-ы из окружения
	orderURL := os.Getenv("ORDER_SERVICE_URL")
	subURL := os.Getenv("SUBSCRIPTION_SERVICE_URL")
	port := os.Getenv("PAYMENT_SERVICE_PORT")
	if port == "" {
		port = "8005"
	}

	// Создаём хендлер, передаём адреса других сервисов
	paymentHandler := handler.NewPaymentHandler(orderURL, subURL)

	// Регистрируем единственный endpoint
	http.HandleFunc("/payments", paymentHandler.Pay)

	// Запускаем HTTP-сервер
	addr := ":" + port
	server := &http.Server{
		Addr:           addr,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	log.Printf("Payment Service is running on %s …\n", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Payment Service failed: %v", err)
	}
}
