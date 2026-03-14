package store

import (
	"path/filepath"
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestOpenAndMigrate(t *testing.T) {
	s := openTestStore(t)

	// Verify all tables exist by querying sqlite_master.
	tables := []string{
		"topics", "progress", "quiz_questions", "spaced_repetition",
		"daily_activity", "confidence_ratings", "mock_sessions",
		"mock_session_scores", "star_stories", "study_plans", "question_attempts",
	}
	for _, tbl := range tables {
		var name string
		err := s.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", tbl).Scan(&name)
		if err != nil {
			t.Errorf("table %s not found: %v", tbl, err)
		}
	}

	// Opening again should be idempotent (IF NOT EXISTS).
	s2, err := Open(s.dbPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	s2.Close()
}

func TestTopicCRUD(t *testing.T) {
	s := openTestStore(t)

	// Create topic.
	topic, err := s.CreateTopic("go", "concurrency", "goroutines", "medium", "/go/concurrency/goroutines.md")
	if err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}
	if topic.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if topic.Module != "go" || topic.Category != "concurrency" || topic.Name != "goroutines" {
		t.Fatalf("unexpected topic fields: %+v", topic)
	}
	if topic.Status != "not_started" {
		t.Fatalf("expected not_started, got %s", topic.Status)
	}

	// Get by ID.
	got, err := s.GetTopic(topic.ID)
	if err != nil {
		t.Fatalf("GetTopic: %v", err)
	}
	if got.Name != "goroutines" {
		t.Fatalf("expected goroutines, got %s", got.Name)
	}

	// Get by name.
	got2, err := s.GetTopicByName("go", "concurrency", "goroutines")
	if err != nil {
		t.Fatalf("GetTopicByName: %v", err)
	}
	if got2.ID != topic.ID {
		t.Fatalf("expected ID %d, got %d", topic.ID, got2.ID)
	}

	// Duplicate returns existing.
	dup, err := s.CreateTopic("go", "concurrency", "goroutines", "hard", "/different.md")
	if err != nil {
		t.Fatalf("CreateTopic duplicate: %v", err)
	}
	if dup.ID != topic.ID {
		t.Fatalf("expected same ID %d for duplicate, got %d", topic.ID, dup.ID)
	}

	// Create another topic in same module.
	_, err = s.CreateTopic("go", "concurrency", "channels", "medium", "")
	if err != nil {
		t.Fatalf("CreateTopic channels: %v", err)
	}

	// List all.
	all, err := s.ListTopics("", "")
	if err != nil {
		t.Fatalf("ListTopics all: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 topics, got %d", len(all))
	}

	// List by module.
	byMod, err := s.ListTopics("go", "")
	if err != nil {
		t.Fatalf("ListTopics by module: %v", err)
	}
	if len(byMod) != 2 {
		t.Fatalf("expected 2 topics for go module, got %d", len(byMod))
	}

	// Update status.
	if err := s.UpdateTopicStatus(topic.ID, "in_progress"); err != nil {
		t.Fatalf("UpdateTopicStatus: %v", err)
	}
	updated, err := s.GetTopic(topic.ID)
	if err != nil {
		t.Fatalf("GetTopic after update: %v", err)
	}
	if updated.Status != "in_progress" {
		t.Fatalf("expected in_progress, got %s", updated.Status)
	}

	// List by status.
	inProg, err := s.ListTopics("", "in_progress")
	if err != nil {
		t.Fatalf("ListTopics by status: %v", err)
	}
	if len(inProg) != 1 {
		t.Fatalf("expected 1 in_progress topic, got %d", len(inProg))
	}
}

func TestQuizAndDrill(t *testing.T) {
	s := openTestStore(t)

	topic, err := s.CreateTopic("go", "basics", "types", "easy", "")
	if err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}

	// Create questions.
	q1, err := s.CreateQuizQuestion(topic.ID, "easy", "What is an int?", "A numeric type", "Built-in integer", "manual")
	if err != nil {
		t.Fatalf("CreateQuizQuestion: %v", err)
	}
	q2, err := s.CreateQuizQuestion(topic.ID, "medium", "What is a slice?", "A dynamic array", "Backed by array", "manual")
	if err != nil {
		t.Fatalf("CreateQuizQuestion q2: %v", err)
	}

	// List questions.
	qs, err := s.ListQuestions(topic.ID)
	if err != nil {
		t.Fatalf("ListQuestions: %v", err)
	}
	if len(qs) != 2 {
		t.Fatalf("expected 2 questions, got %d", len(qs))
	}

	// Pick next — both have 0 attempts, should return first by ID.
	next, err := s.PickNextQuestion(topic.ID)
	if err != nil {
		t.Fatalf("PickNextQuestion: %v", err)
	}
	if next.ID != q1.ID {
		t.Fatalf("expected q1 (ID %d), got %d", q1.ID, next.ID)
	}

	// Record progress and attempt for q1.
	prog, err := s.RecordProgress(topic.ID, 80, 1, 1, 30, "good session")
	if err != nil {
		t.Fatalf("RecordProgress: %v", err)
	}

	_, err = s.RecordAttempt(q1.ID, prog.ID, true, 15, "correct answer")
	if err != nil {
		t.Fatalf("RecordAttempt: %v", err)
	}

	// Now pick next should return q2 (fewest attempts).
	next2, err := s.PickNextQuestion(topic.ID)
	if err != nil {
		t.Fatalf("PickNextQuestion after attempt: %v", err)
	}
	if next2.ID != q2.ID {
		t.Fatalf("expected q2 (ID %d) as least-attempted, got %d", q2.ID, next2.ID)
	}
}

func TestSpacedRepetition(t *testing.T) {
	s := openTestStore(t)

	topic, err := s.CreateTopic("system-design", "patterns", "cqrs", "hard", "")
	if err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}

	tomorrow := time.Now().Add(24 * time.Hour)

	// Upsert — create.
	sr, err := s.UpsertSpacedRep(topic.ID, tomorrow, 1, 2.5, 1)
	if err != nil {
		t.Fatalf("UpsertSpacedRep: %v", err)
	}
	if sr.TopicID != topic.ID || sr.IntervalDays != 1 {
		t.Fatalf("unexpected spaced rep: %+v", sr)
	}

	// Upsert — update.
	nextWeek := time.Now().Add(7 * 24 * time.Hour)
	sr2, err := s.UpsertSpacedRep(topic.ID, nextWeek, 7, 2.6, 2)
	if err != nil {
		t.Fatalf("UpsertSpacedRep update: %v", err)
	}
	if sr2.IntervalDays != 7 || sr2.RepetitionCount != 2 {
		t.Fatalf("expected updated values, got: %+v", sr2)
	}

	// Get.
	got, err := s.GetSpacedRep(topic.ID)
	if err != nil {
		t.Fatalf("GetSpacedRep: %v", err)
	}
	if got.IntervalDays != 7 {
		t.Fatalf("expected interval 7, got %d", got.IntervalDays)
	}

	// Due reviews — next week item should not be due now.
	due, err := s.GetDueReviews(time.Now())
	if err != nil {
		t.Fatalf("GetDueReviews: %v", err)
	}
	if len(due) != 0 {
		t.Fatalf("expected 0 due reviews, got %d", len(due))
	}

	// Due reviews — check far future.
	dueFuture, err := s.GetDueReviews(nextWeek.Add(24 * time.Hour))
	if err != nil {
		t.Fatalf("GetDueReviews future: %v", err)
	}
	if len(dueFuture) != 1 {
		t.Fatalf("expected 1 due review, got %d", len(dueFuture))
	}
}

func TestModuleStats(t *testing.T) {
	s := openTestStore(t)

	// Create topics with different statuses.
	t1, _ := s.CreateTopic("go", "basics", "variables", "easy", "")
	t2, _ := s.CreateTopic("go", "basics", "functions", "easy", "")
	_, _ = s.CreateTopic("go", "advanced", "reflection", "hard", "")

	s.UpdateTopicStatus(t1.ID, "completed")
	s.UpdateTopicStatus(t2.ID, "in_progress")
	// t3 stays not_started

	// Add some progress.
	s.RecordProgress(t1.ID, 90, 5, 4, 300, "")
	s.RecordProgress(t2.ID, 60, 3, 2, 180, "")

	stats, err := s.GetModuleStats("go")
	if err != nil {
		t.Fatalf("GetModuleStats: %v", err)
	}

	if stats.TopicCount != 3 {
		t.Fatalf("expected 3 topics, got %d", stats.TopicCount)
	}
	if stats.CompletedCount != 1 {
		t.Fatalf("expected 1 completed, got %d", stats.CompletedCount)
	}
	if stats.InProgressCount != 1 {
		t.Fatalf("expected 1 in_progress, got %d", stats.InProgressCount)
	}

	// Completion should be ~33.33%.
	if stats.CompletionPct < 33 || stats.CompletionPct > 34 {
		t.Fatalf("expected ~33%% completion, got %.2f%%", stats.CompletionPct)
	}
	if stats.TotalTimeSeconds != 480 {
		t.Fatalf("expected 480 seconds total, got %d", stats.TotalTimeSeconds)
	}
	if stats.AvgScore < 74 || stats.AvgScore > 76 {
		t.Fatalf("expected avg score ~75, got %.2f", stats.AvgScore)
	}

	// Empty module stats.
	empty, err := s.GetModuleStats("python")
	if err != nil {
		t.Fatalf("GetModuleStats empty: %v", err)
	}
	if empty.TopicCount != 0 || empty.CompletionPct != 0 {
		t.Fatalf("expected empty stats, got: %+v", empty)
	}
}

func TestStreak(t *testing.T) {
	s := openTestStore(t)

	// No activity — streak is 0.
	streak, err := s.GetStreak()
	if err != nil {
		t.Fatalf("GetStreak empty: %v", err)
	}
	if streak != 0 {
		t.Fatalf("expected 0 streak, got %d", streak)
	}

	// Add 3 consecutive days ending today.
	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	twoDaysAgo := time.Now().AddDate(0, 0, -2).Format("2006-01-02")

	s.UpsertDailyActivity(today, "go", 300, 1, 5, 80)
	s.UpsertDailyActivity(yesterday, "go", 200, 1, 3, 70)
	s.UpsertDailyActivity(twoDaysAgo, "go", 100, 1, 2, 60)

	streak, err = s.GetStreak()
	if err != nil {
		t.Fatalf("GetStreak: %v", err)
	}
	if streak != 3 {
		t.Fatalf("expected 3 day streak, got %d", streak)
	}

	// Add a gap — 5 days ago (missing 3 and 4 days ago).
	fiveDaysAgo := time.Now().AddDate(0, 0, -5).Format("2006-01-02")
	s.UpsertDailyActivity(fiveDaysAgo, "go", 100, 1, 1, 50)

	streak, err = s.GetStreak()
	if err != nil {
		t.Fatalf("GetStreak with gap: %v", err)
	}
	if streak != 3 {
		t.Fatalf("expected 3 day streak (gap should break it), got %d", streak)
	}
}

func TestMockSession(t *testing.T) {
	s := openTestStore(t)

	// Create session.
	ms, err := s.CreateMockSession("behavioral", "Senior Go Developer")
	if err != nil {
		t.Fatalf("CreateMockSession: %v", err)
	}
	if ms.Type != "behavioral" || ms.CompletedAt != nil {
		t.Fatalf("unexpected session: %+v", ms)
	}

	// Add scores.
	if err := s.AddMockScore(ms.ID, "communication", 8.5); err != nil {
		t.Fatalf("AddMockScore: %v", err)
	}
	if err := s.AddMockScore(ms.ID, "problem_solving", 7.0); err != nil {
		t.Fatalf("AddMockScore 2: %v", err)
	}

	scores, err := s.GetMockScores(ms.ID)
	if err != nil {
		t.Fatalf("GetMockScores: %v", err)
	}
	if len(scores) != 2 {
		t.Fatalf("expected 2 scores, got %d", len(scores))
	}

	// Complete session.
	if err := s.CompleteMockSession(ms.ID, 7.75, `{"strengths":["clear communication"]}`); err != nil {
		t.Fatalf("CompleteMockSession: %v", err)
	}

	completed, err := s.GetMockSession(ms.ID)
	if err != nil {
		t.Fatalf("GetMockSession completed: %v", err)
	}
	if completed.CompletedAt == nil {
		t.Fatal("expected completed_at to be set")
	}
	if completed.OverallScore != 7.75 {
		t.Fatalf("expected score 7.75, got %f", completed.OverallScore)
	}

	// List sessions.
	all, err := s.ListMockSessions("")
	if err != nil {
		t.Fatalf("ListMockSessions: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 session, got %d", len(all))
	}

	// List by type.
	behavioral, err := s.ListMockSessions("behavioral")
	if err != nil {
		t.Fatalf("ListMockSessions behavioral: %v", err)
	}
	if len(behavioral) != 1 {
		t.Fatalf("expected 1 behavioral session, got %d", len(behavioral))
	}

	technical, err := s.ListMockSessions("technical")
	if err != nil {
		t.Fatalf("ListMockSessions technical: %v", err)
	}
	if len(technical) != 0 {
		t.Fatalf("expected 0 technical sessions, got %d", len(technical))
	}
}

func TestStudyPlan(t *testing.T) {
	s := openTestStore(t)

	// No active plan.
	_, err := s.GetActivePlan()
	if err == nil {
		t.Fatal("expected error for no active plan")
	}

	// Create first plan.
	p1, err := s.CreatePlan("Senior Go Developer", "2026-06-01", `{"weeks":[]}`)
	if err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}
	if !p1.Active || p1.TargetRole != "Senior Go Developer" {
		t.Fatalf("unexpected plan: %+v", p1)
	}

	// Get active.
	active, err := s.GetActivePlan()
	if err != nil {
		t.Fatalf("GetActivePlan: %v", err)
	}
	if active.ID != p1.ID {
		t.Fatalf("expected plan ID %d, got %d", p1.ID, active.ID)
	}

	// Create second plan — first should be deactivated.
	p2, err := s.CreatePlan("Staff Engineer", "2026-12-01", `{"weeks":[1,2]}`)
	if err != nil {
		t.Fatalf("CreatePlan 2: %v", err)
	}

	active2, err := s.GetActivePlan()
	if err != nil {
		t.Fatalf("GetActivePlan after second: %v", err)
	}
	if active2.ID != p2.ID {
		t.Fatalf("expected active plan ID %d, got %d", p2.ID, active2.ID)
	}

	// Verify first plan is deactivated.
	var firstActive int
	err = s.db.QueryRow("SELECT active FROM study_plans WHERE id = ?", p1.ID).Scan(&firstActive)
	if err != nil {
		t.Fatalf("query first plan: %v", err)
	}
	if firstActive != 0 {
		t.Fatalf("expected first plan deactivated, got active=%d", firstActive)
	}

	// Update plan.
	if err := s.UpdatePlan(p2.ID, `{"weeks":[1,2,3]}`); err != nil {
		t.Fatalf("UpdatePlan: %v", err)
	}
	updated, err := s.GetActivePlan()
	if err != nil {
		t.Fatalf("GetActivePlan after update: %v", err)
	}
	if updated.PlanJSON != `{"weeks":[1,2,3]}` {
		t.Fatalf("expected updated plan JSON, got %s", updated.PlanJSON)
	}
}

func TestConfidenceGaps(t *testing.T) {
	s := openTestStore(t)

	t1, _ := s.CreateTopic("go", "basics", "interfaces", "medium", "")
	t2, _ := s.CreateTopic("go", "basics", "structs", "easy", "")

	// Topic 1: overconfident (self=9, actual=5 → gap=4).
	s.AddConfidenceRating(t1.ID, 9.0, 5.0)
	s.AddConfidenceRating(t1.ID, 9.0, 5.0)

	// Topic 2: well-calibrated (self=7, actual=7 → gap=0).
	s.AddConfidenceRating(t2.ID, 7.0, 7.0)

	// Gap threshold 2.0 — only topic 1 should appear.
	gaps, err := s.GetConfidenceGaps(2.0)
	if err != nil {
		t.Fatalf("GetConfidenceGaps: %v", err)
	}
	if len(gaps) != 1 {
		t.Fatalf("expected 1 gap, got %d", len(gaps))
	}
	if gaps[0].TopicID != t1.ID {
		t.Fatalf("expected topic %d, got %d", t1.ID, gaps[0].TopicID)
	}
	if gaps[0].Gap < 3.9 || gaps[0].Gap > 4.1 {
		t.Fatalf("expected gap ~4.0, got %.2f", gaps[0].Gap)
	}

	// Gap threshold 5.0 — nothing should match.
	noGaps, err := s.GetConfidenceGaps(5.0)
	if err != nil {
		t.Fatalf("GetConfidenceGaps high threshold: %v", err)
	}
	if len(noGaps) != 0 {
		t.Fatalf("expected 0 gaps above 5.0, got %d", len(noGaps))
	}
}
