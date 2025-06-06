package utils

import (
	"bytes"
	"cleaning-app/order-service/internal/config"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func SendGamificationXP(cfg *config.Config, userID string, xp int) {
	go func() {
		// Создаём собственный контекст с таймаутом, вместо ctx (который отменится при выходе из HTTP-хендлера)
		innerCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		url := fmt.Sprintf("%s/users/gamification/add-xp", cfg.UserManagementURL)

		payload := map[string]interface{}{
			"user_id": userID,
			"xp":      xp,
		}
		data, _ := json.Marshal(payload)

		req, err := http.NewRequestWithContext(innerCtx, http.MethodPost, url, bytes.NewBuffer(data))
		if err != nil {
			fmt.Printf("[GamificationClient] Ошибка при создании запроса: %v\n", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 3 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("[GamificationClient] Не удалось отправить XP (userID=%s): %v\n", userID, err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 300 {
			fmt.Printf("[GamificationClient] Получен статус %d при отправке XP пользователю %s\n", resp.StatusCode, userID)
			return
		}
	}()
}
