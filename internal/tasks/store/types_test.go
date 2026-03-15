package store

import "testing"

func TestSubstepOrder(t *testing.T) {
	order := SubstepOrder()
	if len(order) != 6 {
		t.Fatalf("SubstepOrder() length = %d, want 6", len(order))
	}
	expected := []Substep{
		SubstepTDD, SubstepImplementing, SubstepReviewing,
		SubstepQATest, SubstepE2ETest, SubstepSecurityReview,
	}
	for i, s := range expected {
		if order[i] != s {
			t.Errorf("SubstepOrder()[%d] = %q, want %q", i, order[i], s)
		}
	}
}

func TestSubstepNext(t *testing.T) {
	next, ok := SubstepTDD.Next()
	if !ok || next != SubstepImplementing {
		t.Errorf("TDD.Next() = (%q, %v), want (%q, true)", next, ok, SubstepImplementing)
	}

	_, ok = SubstepSecurityReview.Next()
	if ok {
		t.Error("SecurityReview.Next() should return false")
	}
}

func TestSubstepValid(t *testing.T) {
	if !Substep("tdd").Valid() {
		t.Error("tdd should be valid")
	}
	if !Substep("").Valid() {
		t.Error("empty string should be valid")
	}
	if Substep("invalid").Valid() {
		t.Error("invalid should not be valid")
	}
}
