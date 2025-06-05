package services

import (
	"reflect"
	"testing"
	"time"

	"cleaning-app/subscription-service/internal/models"
)

var svc = &SubscriptionService{}

func TestNextDates_Weekly(t *testing.T) {

	spec := models.ScheduleSpec{
		Frequency:  models.Weekly,
		DaysOfWeek: []string{"Mon", "Wed"},
	}
	from := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC) // 1 июня 2025 — воскресенье
	until := time.Date(2025, 6, 10, 0, 0, 0, 0, time.UTC)

	got := svc.NextDates(spec, from, until)
	want := []time.Time{
		time.Date(2025, 6, 2, 0, 0, 0, 0, time.UTC), // Понедельник
		time.Date(2025, 6, 4, 0, 0, 0, 0, time.UTC), // Среда
		time.Date(2025, 6, 9, 0, 0, 0, 0, time.UTC), // Понедельник
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("NextDates weekly = %v, want %v", got, want)
	}
}

func TestNextDates_BiWeekly(t *testing.T) {
	spec := models.ScheduleSpec{
		Frequency:   models.BiWeekly,
		DaysOfWeek:  []string{"Tue"},
		WeekNumbers: []int{1, 3},
	}
	from := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2025, 6, 30, 0, 0, 0, 0, time.UTC)

	got := svc.NextDates(spec, from, until)
	// 1-я неделя июня: Tuesday — 3 июня
	// 3-я неделя июня: Tuesday — 17 июня
	want := []time.Time{
		time.Date(2025, 6, 3, 0, 0, 0, 0, time.UTC),
		time.Date(2025, 6, 17, 0, 0, 0, 0, time.UTC),
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("NextDates biweekly = %v, want %v", got, want)
	}
}

func TestNextDates_TriWeekly(t *testing.T) {

	spec := models.ScheduleSpec{
		Frequency:   models.TriWeekly,
		DaysOfWeek:  []string{"Fri"},
		WeekNumbers: []int{1, 2, 4},
	}
	from := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2025, 6, 30, 0, 0, 0, 0, time.UTC)

	got := svc.NextDates(spec, from, until)
	// Fri 1-я неделя: 6 июня
	// Fri 2-я неделя: 13 июня
	// Fri 4-я неделя: 27 июня
	want := []time.Time{
		time.Date(2025, 6, 6, 0, 0, 0, 0, time.UTC),
		time.Date(2025, 6, 13, 0, 0, 0, 0, time.UTC),
		time.Date(2025, 6, 27, 0, 0, 0, 0, time.UTC),
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("NextDates triweekly = %v, want %v", got, want)
	}
}

func TestNextDates_Monthly(t *testing.T) {
	spec := models.ScheduleSpec{
		Frequency:   models.Monthly,
		DaysOfWeek:  []string{"Sat"},
		WeekNumbers: []int{2},
	}
	from := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2025, 6, 30, 0, 0, 0, 0, time.UTC)

	got := svc.NextDates(spec, from, until)
	// 2-я неделя июня (8–14 июня), Saturday — 14 июня
	want := []time.Time{
		time.Date(2025, 6, 14, 0, 0, 0, 0, time.UTC),
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("NextDates monthly = %v, want %v", got, want)
	}
}
