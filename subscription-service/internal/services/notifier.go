package services

import (
	"cleaning-app/subscription-service/internal/models"
	"context"
	"log"
	"time"
)

// Notifier handles background subscription expiration checks and notifications
type Notifier struct {
	subscriptionService *SubscriptionService
	notificationService NotificationService
}
type NotificationService interface {
	SendSubscriptionNotification(ctx context.Context, sub models.Subscription, event string, data map[string]string) error
}

// NewNotifier creates a new Notifier
func NewNotifier(subscriptionService *SubscriptionService, notificationService NotificationService) *Notifier {
	return &Notifier{
		subscriptionService: subscriptionService,
		notificationService: notificationService,
	}
}

// Start begins the background notification processes
func (n *Notifier) Start(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	go func() {
		// Run immediately on start
		n.checkSubscriptions(ctx)

		for {
			select {
			case <-ticker.C:
				log.Println("[CRON] Проверка подписок...")
				n.checkSubscriptions(ctx)
			case <-ctx.Done():
				ticker.Stop()
				log.Println("[CRON] Notifier shutdown")
				return
			}
		}
	}()
}

// checkSubscriptions handles all subscription-related checks
func (n *Notifier) checkSubscriptions(ctx context.Context) {
	// Check for expiring subscriptions (3 days notice)
	n.sendExpiringNotifications(ctx)

	// Check for expired subscriptions
	n.expireSubscriptions(ctx)
}

// sendExpiringNotifications sends notifications for subscriptions expiring in 3 days
func (n *Notifier) sendExpiringNotifications(ctx context.Context) {
	targetDate := time.Now().Add(72 * time.Hour).Truncate(24 * time.Hour)

	subs, err := n.subscriptionService.FindExpiringOn(ctx, targetDate)
	if err != nil {
		log.Printf("Ошибка при поиске подписок: %v", err)
		return
	}

	for _, sub := range subs {
		expiryInfo := ""
		if sub.NextPlannedDate != nil {
			expiryInfo = sub.NextPlannedDate.Format("2006-01-02")
		} else {
			expiryInfo = "unknown"
		}

		err := n.notificationService.SendSubscriptionNotification(ctx, sub, "expiring_soon", map[string]string{
			"expiry_date": expiryInfo,
		})

		if err != nil {
			log.Printf("Ошибка отправки уведомления клиенту %s: %v", sub.ClientID, err)
		} else {
			log.Printf("Отправлено уведомление об истечении подписки %s клиенту %s", sub.ID.Hex(), sub.ClientID)
		}
	}
}

// expireSubscriptions updates the status of expired subscriptions
func (n *Notifier) expireSubscriptions(ctx context.Context) {
	now := time.Now().Truncate(24 * time.Hour)

	expired, err := n.subscriptionService.FindExpired(ctx, now)
	if err != nil {
		log.Printf("Ошибка при поиске просроченных подписок: %v", err)
		return
	}

	for _, sub := range expired {
		err := n.subscriptionService.UpdateStatus(ctx, sub.ID, models.StatusExpired)
		if err != nil {
			log.Printf("Не удалось обновить подписку %s: %v", sub.ID.Hex(), err)
			continue
		}

		// Log the expiration
		err = LogSubscriptionAction(ctx, nil, sub.ID, sub.ClientID, "expired", nil)
		if err != nil {
			log.Printf("Не удалось записать лог для подписки %s: %v", sub.ID.Hex(), err)
		}

		// Send notification about expiration
		err = n.notificationService.SendSubscriptionNotification(ctx, sub, "expired", nil)
		if err != nil {
			log.Printf("Не удалось отправить уведомление о истечении подписки %s: %v", sub.ID.Hex(), err)
		}

		log.Printf("Подписка %s завершена по сроку", sub.ID.Hex())
	}
}
