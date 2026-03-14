package modules

import (
	"math"
	"testing"
	"time"
)

func TestSM2FailedResetsInterval(t *testing.T) {
	// Quality < 3 should reset interval to 1 and reps to 0.
	for _, q := range []int{0, 1, 2} {
		r := SM2Update(q, 10, 2.5, 5)
		if r.IntervalDays != 1 {
			t.Errorf("quality %d: expected interval 1, got %f", q, r.IntervalDays)
		}
		if r.RepetitionCount != 0 {
			t.Errorf("quality %d: expected reps 0, got %d", q, r.RepetitionCount)
		}
	}
}

func TestSM2FirstSuccessInterval(t *testing.T) {
	// First success (reps 0 → 1): interval = 1.
	r := SM2Update(4, 0, 2.5, 0)
	if r.IntervalDays != 1 {
		t.Errorf("expected interval 1, got %f", r.IntervalDays)
	}
	if r.RepetitionCount != 1 {
		t.Errorf("expected reps 1, got %d", r.RepetitionCount)
	}
}

func TestSM2SecondSuccessInterval(t *testing.T) {
	// Second success (reps 1 → 2): interval = 6.
	r := SM2Update(4, 1, 2.5, 1)
	if r.IntervalDays != 6 {
		t.Errorf("expected interval 6, got %f", r.IntervalDays)
	}
	if r.RepetitionCount != 2 {
		t.Errorf("expected reps 2, got %d", r.RepetitionCount)
	}
}

func TestSM2ThirdSuccessInterval(t *testing.T) {
	// Third success (reps 2 → 3): interval = round(6 * EF).
	ef := 2.5
	r := SM2Update(4, 6, ef, 2)
	expected := math.Round(6 * ef) // 15
	if r.IntervalDays != expected {
		t.Errorf("expected interval %f, got %f", expected, r.IntervalDays)
	}
	if r.RepetitionCount != 3 {
		t.Errorf("expected reps 3, got %d", r.RepetitionCount)
	}
}

func TestSM2EaseFactorFloor(t *testing.T) {
	// Repeated failures should not push EF below 1.3.
	ef := 1.3
	for i := 0; i < 10; i++ {
		r := SM2Update(0, 1, ef, 0)
		ef = r.EaseFactor
	}
	if ef < 1.3 {
		t.Errorf("ease factor fell below 1.3: %f", ef)
	}
}

func TestSM2PerfectQualityRaisesEF(t *testing.T) {
	// Quality 5 with starting EF 2.5 should raise EF.
	r := SM2Update(5, 6, 2.5, 2)
	if r.EaseFactor <= 2.5 {
		t.Errorf("expected EF > 2.5 for perfect quality, got %f", r.EaseFactor)
	}
}

func TestSM2ClampedInput(t *testing.T) {
	// Negative quality clamped to 0.
	r := SM2Update(-5, 1, 2.5, 0)
	if r.IntervalDays != 1 {
		t.Errorf("negative quality: expected interval 1, got %f", r.IntervalDays)
	}
	if r.RepetitionCount != 0 {
		t.Errorf("negative quality: expected reps 0, got %d", r.RepetitionCount)
	}

	// Overflow quality clamped to 5.
	r2 := SM2Update(100, 6, 2.5, 2)
	// Quality 5 is a pass, so reps should increment.
	if r2.RepetitionCount != 3 {
		t.Errorf("overflow quality: expected reps 3, got %d", r2.RepetitionCount)
	}
	// EF should be same as quality=5.
	r3 := SM2Update(5, 6, 2.5, 2)
	if math.Abs(r2.EaseFactor-r3.EaseFactor) > 0.001 {
		t.Errorf("overflow quality EF %f != quality-5 EF %f", r2.EaseFactor, r3.EaseFactor)
	}
}

func TestSM2NextReviewInFuture(t *testing.T) {
	r := SM2Update(4, 6, 2.5, 2)
	if r.NextReview.Before(time.Now()) {
		t.Errorf("expected next review in future, got %v", r.NextReview)
	}
}
