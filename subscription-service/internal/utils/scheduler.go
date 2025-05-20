package utils

import (
	"context"
	"time"
)

func StartSubscriptionScheduler(ctx context.Context, task func(context.Context)) {
	ticker := time.NewTicker(24 * time.Hour)
	go func() {
		for {
			select {
			case <-ticker.C:
				task(ctx)
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

func DayMatch(days []string, today string) bool {
	for _, d := range days {
		if d == today {
			return true
		}
	}
	return false
}

func NextValidDate(days []string, from time.Time) time.Time {
	for i := 1; i <= 14; i++ {
		next := from.AddDate(0, 0, i)
		if DayMatch(days, next.Weekday().String()) {
			return next
		}
	}
	return from
}

func IsToday(t time.Time) bool {
	now := time.Now()
	return t.Year() == now.Year() && t.YearDay() == now.YearDay()
}
