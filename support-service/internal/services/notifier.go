package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type NotifierService struct {
	BaseURL string
}

func NewNotifierService(baseURL string) *NotifierService {
	return &NotifierService{BaseURL: baseURL}
}

func (n *NotifierService) SendNotification(userID, message string) error {
	payload := map[string]string{
		"user_id": userID,
		"message": message,
	}
	body, _ := json.Marshal(payload)
	resp, err := http.Post(fmt.Sprintf("%s/send", n.BaseURL), "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
