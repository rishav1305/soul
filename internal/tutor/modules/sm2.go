package modules

import (
	"math"
	"time"
)

// SM2Result holds the output of the SM-2 spaced repetition algorithm.
type SM2Result struct {
	IntervalDays    float64
	EaseFactor      float64
	RepetitionCount int
	NextReview      time.Time
}

// SM2Update applies the SM-2 algorithm.
// quality: 0-5 (0=blackout, 5=perfect).
func SM2Update(quality int, currentInterval float64, currentEF float64, currentReps int) SM2Result {
	if quality < 0 {
		quality = 0
	}
	if quality > 5 {
		quality = 5
	}

	var interval float64
	var reps int
	ef := currentEF

	if quality < 3 {
		// Failed review — reset.
		interval = 1
		reps = 0
	} else {
		reps = currentReps + 1
		switch reps {
		case 1:
			interval = 1
		case 2:
			interval = 6
		default:
			interval = math.Round(currentInterval * ef)
		}
	}

	// Update ease factor.
	ef = ef + (0.1 - float64(5-quality)*(0.08+float64(5-quality)*0.02))
	if ef < 1.3 {
		ef = 1.3
	}

	return SM2Result{
		IntervalDays:    interval,
		EaseFactor:      ef,
		RepetitionCount: reps,
		NextReview:      time.Now().Add(time.Duration(interval*24) * time.Hour),
	}
}
