package utils

import (
	"context"
	"fmt"
	"time"
)

type Scheduler interface {
	ProcessDailyOrders(ctx context.Context)
}

func StartScheduler(svc Scheduler) {
	go func() {
		for {
			now := time.Now().UTC()
			// определяем, сколько ждать до следующей 02:00 UTC
			nextRun := time.Date(now.Year(), now.Month(), now.Day(), 2, 0, 0, 0, time.UTC)
			if now.After(nextRun) {
				nextRun = nextRun.Add(24 * time.Hour)
			}
			wait := nextRun.Sub(now)
			time.Sleep(wait)

			fmt.Println("Scheduler: ProcessDailyOrders at", time.Now().UTC())
			svc.ProcessDailyOrders(context.Background())
			// далее снова ждём до следующего 02:00
		}
	}()
}
