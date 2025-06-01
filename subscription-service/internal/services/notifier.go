package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"cleaning-app/subscription-service/internal/models"
	"cleaning-app/subscription-service/internal/utils"
)

// Notifier отвечает за уведомления об истечении и завершении подписок.
type Notifier struct {
	subscriptionService *SubscriptionService
	notifyClient        *utils.NotificationServiceClient
}

// NewNotifier создаёт новый Notifier:
// - subscriptionService должен реализовывать FindExpiringOn, FindExpired и UpdateStatus
// - notifyClient — это utils.NewNotificationClient(...)
func NewNotifier(
	subscriptionService *SubscriptionService,
	notifyClient *utils.NotificationServiceClient,
) *Notifier {
	return &Notifier{
		subscriptionService: subscriptionService,
		notifyClient:        notifyClient,
	}
}

// Start запускает фоновые задачи уведомлений каждые сутки.
func (n *Notifier) Start(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	go func() {
		// Сразу при старте
		n.runChecks(ctx)

		for {
			select {
			case <-ticker.C:
				log.Println("[NOTIFIER] Running subscription checks...")
				n.runChecks(ctx)
			case <-ctx.Done():
				ticker.Stop()
				log.Println("[NOTIFIER] Shutdown")
				return
			}
		}
	}()
}

// runChecks выполняет обе проверки: expiring (за 3 дня) и expired (сразу после окончания).
func (n *Notifier) runChecks(ctx context.Context) {
	n.sendExpiringNotifications(ctx)
	n.sendExpiredNotifications(ctx)
}

// sendExpiringNotifications находит подписки, у которых EndDate == now+3d, и шлёт “скоро истекает”.
func (n *Notifier) sendExpiringNotifications(ctx context.Context) {
	targetDate := time.Now().Add(72 * time.Hour).Truncate(24 * time.Hour)

	subs, err := n.subscriptionService.FindExpiringOn(ctx, targetDate)
	if err != nil {
		log.Printf("[NOTIFIER] FindExpiringOn error: %v", err)
		return
	}

	for _, sub := range subs {
		msg := fmt.Sprintf(
			"Ваша подписка %s истекает %s. Хотите продлить или отменить?",
			sub.ID.Hex(),
			sub.EndDate.Format("2006-01-02"),
		)
		if err := n.notifyClient.SendNotification(ctx, sub.UserID.Hex(), msg); err != nil {
			log.Printf("[NOTIFIER] Failed to notify expiring for sub %s: %v", sub.ID.Hex(), err)
		} else {
			log.Printf("[NOTIFIER] Notified expiring for sub %s to user %s", sub.ID.Hex(), sub.UserID.Hex())
		}
	}
}

// sendExpiredNotifications находит уже просроченные подписки и помечает их expired, затем уведомляет.
func (n *Notifier) sendExpiredNotifications(ctx context.Context) {
	now := time.Now().Truncate(24 * time.Hour)

	subs, err := n.subscriptionService.FindExpired(ctx, now)
	if err != nil {
		log.Printf("[NOTIFIER] FindExpired error: %v", err)
		return
	}

	for _, sub := range subs {
		// обновляем статус в БД
		if err := n.subscriptionService.UpdateStatus(ctx, sub.ID, models.StatusExpired); err != nil {
			log.Printf("[NOTIFIER] Failed to update status for sub %s: %v", sub.ID.Hex(), err)
			continue
		}

		msg := fmt.Sprintf("Ваша подписка %s завершена. Хотите продлить или отменить?", sub.ID.Hex())
		if err := n.notifyClient.SendNotification(ctx, sub.UserID.Hex(), msg); err != nil {
			log.Printf("[NOTIFIER] Failed to notify expired for sub %s: %v", sub.ID.Hex(), err)
		} else {
			log.Printf("[NOTIFIER] Notified expired for sub %s to user %s", sub.ID.Hex(), sub.UserID.Hex())
		}
	}
}
