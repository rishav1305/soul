package store

import (
	"database/sql"
	"testing"
)

func TestAddLeadIfNotExists_New(t *testing.T) {
	s := newTestStore(t)
	tsID := int64(123)
	lead := Lead{
		JobTitle:     "Go Engineer",
		Company:      "Acme",
		TheirStackID: &tsID,
	}
	id, created, err := s.AddLeadIfNotExists(lead)
	if err != nil {
		t.Fatalf("AddLeadIfNotExists: %v", err)
	}
	if !created {
		t.Error("expected created=true for new lead")
	}
	if id <= 0 {
		t.Errorf("expected positive id, got %d", id)
	}
}

func TestAddLeadIfNotExists_Duplicate(t *testing.T) {
	s := newTestStore(t)
	tsID := int64(456)
	lead := Lead{
		JobTitle:     "Go Engineer",
		Company:      "Acme",
		TheirStackID: &tsID,
	}
	id1, created1, _ := s.AddLeadIfNotExists(lead)
	if !created1 {
		t.Fatal("expected first insert to be new")
	}

	// Insert again with same theirstack_id.
	id2, created2, err := s.AddLeadIfNotExists(lead)
	if err != nil {
		t.Fatalf("AddLeadIfNotExists (duplicate): %v", err)
	}
	if created2 {
		t.Error("expected created=false for duplicate")
	}
	if id2 != id1 {
		t.Errorf("expected same ID %d, got %d", id1, id2)
	}
}

func TestAddLeadIfNotExists_NilTheirStackID(t *testing.T) {
	s := newTestStore(t)
	lead := Lead{
		JobTitle: "Frontend Dev",
		Company:  "WidgetCo",
		// TheirStackID is nil — always inserts
	}
	id1, created1, _ := s.AddLeadIfNotExists(lead)
	id2, created2, _ := s.AddLeadIfNotExists(lead)

	if !created1 || !created2 {
		t.Error("expected both inserts to be new when theirstack_id is nil")
	}
	if id1 == id2 {
		t.Error("expected different IDs for two nil-theirstack inserts")
	}
}

func TestNullStr(t *testing.T) {
	tests := []struct {
		input sql.NullString
		want  string
	}{
		{sql.NullString{String: "hello", Valid: true}, "hello"},
		{sql.NullString{Valid: false}, ""},
	}
	for _, tc := range tests {
		got := nullStr(tc.input)
		if got != tc.want {
			t.Errorf("nullStr(%+v) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestDB(t *testing.T) {
	s := newTestStore(t)
	db := s.DB()
	if db == nil {
		t.Fatal("expected non-nil *sql.DB")
	}
	if err := db.Ping(); err != nil {
		t.Errorf("Ping: %v", err)
	}
}
