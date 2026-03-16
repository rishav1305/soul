package server

import (
	"testing"

	"github.com/rishav1305/soul-v2/internal/chat/metrics"
)

func TestBuildResilientPillar_LiveConstraints(t *testing.T) {
	report := &metrics.ConnectionHealthReport{
		TotalConnects:       100,
		TotalDisconnects:    100,
		AbnormalDisconnects: 0,
		ReconnectSuccesses:  5,
		ReconnectFailures:   0,
	}

	result := buildResilientPillar(report)

	staticCount := 0
	for _, c := range result.Constraints {
		if c.Status == "static" {
			staticCount++
		}
	}
	if staticCount == len(result.Constraints) {
		t.Error("all constraints are static — expected live constraints")
	}

	for _, c := range result.Constraints {
		if c.Name == "chat-drop-rate" && c.Status != "pass" {
			t.Errorf("expected chat-drop-rate to pass, got %s", c.Status)
		}
	}
}

func TestBuildTransparentPillar_IncludesReliabilitySignals(t *testing.T) {
	usage := &metrics.UsageReport{
		TotalEvents: 10,
		Actions:     map[string]int{"chat.send": 5},
	}
	ch := &metrics.ConnectionHealthReport{
		TotalConnects:       5,
		AuthFailures:        1,
		AuthSuccesses:       4,
		TotalDisconnects:    5,
		AbnormalDisconnects: 0,
	}

	result := buildTransparentPillar(usage, ch)

	constraintNames := make(map[string]bool)
	for _, c := range result.Constraints {
		constraintNames[c.Name] = true
	}

	required := []string{"event-tracking", "usage-tracking", "auth-event-coverage", "ws-lifecycle-coverage"}
	for _, name := range required {
		if !constraintNames[name] {
			t.Errorf("missing required constraint: %s", name)
		}
	}
}
