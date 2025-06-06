package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type GamificationStatus struct {
	UserID        primitive.ObjectID `json:"user_id"`
	XPTotal       int                `json:"xp_total"`
	CurrentLevel  int                `json:"current_level"`
	XPToNextLevel int                `json:"xp_to_next_level"`
}

func CalculateLevel(xpTotal int) (level int, xpToNext int) {
	// Пример: пороги {Level:XP_needed}
	// Lvl 1: 0–49 XP, Lvl 2: 50–149 XP, Lvl 3: 150–299 XP, Lvl 4: 300–499 XP, Lvl 5: 500+ XP.
	// Можно вынести в константы или читать из конфига.
	thresholds := []int{0, 50, 150, 300, 500, 1_000_000} // последний «бесконечный» порог
	lvl := 1
	for i := 1; i < len(thresholds)-1; i++ {
		if xpTotal >= thresholds[i] && xpTotal < thresholds[i+1] {
			lvl = i + 1
			break
		}
	}
	// Текущий уровень найден, теперь сколько XP до следующего:
	if lvl < len(thresholds) {
		xpToNext = thresholds[lvl] - xpTotal
	} else {
		xpToNext = 0
	}
	return lvl, xpToNext
}
