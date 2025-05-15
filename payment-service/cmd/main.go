package main

import (
	"cleaning-app/payment-service/internal/handler"
	"log"
	"net/http"
)

func main() {
	h := handler.NewHandler()
	http.HandleFunc("/api/payments", h.Pay)

	log.Println("Mock Payment Service listening on :8005")
	log.Fatal(http.ListenAndServe("0.0.0.0:8005", nil))
}
