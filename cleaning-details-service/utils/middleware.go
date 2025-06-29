package utils

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"strings"
	"time"
)

type AuthData struct {
	UserID        string `json:"user_id"`
	Role          string `json:"role"`
	ResetRequired bool   `json:"reset_required"`
	Banned        bool   `json:"banned"`
}

func AuthMiddleware(authURL string) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Printf("[AUTH] Using auth service at: %s", authURL)

		token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token missing"})
			return
		}

		req, err := http.NewRequest("GET", authURL+"/auth/validate", nil)
		if err != nil {
			log.Printf("[AUTH] Failed to create request: %v", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}

		req.Header.Set("Authorization", "Bearer "+token)
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("[AUTH] Error calling auth service: %v", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Auth service unavailable"})
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("[AUTH] Auth service returned status: %d", resp.StatusCode)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token invalid"})
			return
		}

		var data AuthData
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			log.Printf("[AUTH] Failed to decode auth response: %v", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Invalid response from auth service"})
			return
		}

		if data.Banned {
			log.Printf("[AUTH] Access denied for banned user: %s", data.UserID)
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Account is banned"})
			return
		}

		if data.ResetRequired {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Password reset required"})
			return
		}

		log.Printf("[AUTH] Authenticated user: %s with role: %s", data.UserID, data.Role)
		c.Set("userId", data.UserID)
		c.Set("role", data.Role)
		c.Next()
	}
}

func RequireRoles(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetString("role")
		for _, allowed := range roles {
			if role == allowed {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Access denied"})
	}
}
