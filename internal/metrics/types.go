package metrics

import (
	"encoding/json"
	"errors"
	"time"
)

// Event type constants — dot-namespaced event categories.
const (
	// System events
	EventSystemStart  = "system.start"
	EventSystemStop   = "system.stop"
	EventSystemSample = "system.sample"
	EventSystemDB     = "system.db"

	// WebSocket events
	EventWSConnect     = "ws.connect"
	EventWSDisconnect  = "ws.disconnect"
	EventWSStreamStart = "ws.stream.start"
	EventWSStreamToken = "ws.stream.token"
	EventWSStreamEnd   = "ws.stream.end"

	// API events
	EventAPIRequest = "api.request"
	EventAPIError   = "api.error"

	// Auth events
	EventOAuthRefresh = "oauth.refresh"
	EventOAuthReload  = "oauth.reload"

	// Pipeline events
	EventStepStart    = "step.start"
	EventStepComplete = "step.complete"
	EventStepBlocked  = "step.blocked"

	// Gate events
	EventGateRun   = "gate.run"
	EventGatePass  = "gate.pass"
	EventGateFail  = "gate.fail"
	EventGateRetry = "gate.retry"

	// Visual events
	EventScreenshotTaken = "screenshot.taken"
	EventScreenshotMiss  = "screenshot.miss"

	// Override events
	EventOverrideError   = "override.error"
	EventOverrideAccept  = "override.accept"
	EventOverrideQuality = "override.quality"

	// Cost events
	EventCostStep = "cost.step"
)

// Error taxonomy constants — categories for classifying errors.
const (
	ErrorSyntax        = "syntax"
	ErrorDependency    = "dependency"
	ErrorLogic         = "logic"
	ErrorUIUX          = "ui_ux"
	ErrorRegression    = "regression"
	ErrorSecurity      = "security"
	ErrorPerformance   = "performance"
	ErrorSpecDrift     = "spec_drift"
	ErrorTestQuality   = "test_quality"
	ErrorFalsePositive = "false_positive"
)

// Event represents a single metrics event in the JSONL log.
type Event struct {
	Timestamp time.Time              `json:"ts"`
	EventType string                 `json:"event"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// Validate checks that the event has a non-empty type and non-zero timestamp.
func (e Event) Validate() error {
	if e.EventType == "" {
		return errors.New("event type must not be empty")
	}
	if e.Timestamp.IsZero() {
		return errors.New("event timestamp must not be zero")
	}
	return nil
}

// MarshalJSON serializes the event with RFC3339Nano timestamp.
func (e Event) MarshalJSON() ([]byte, error) {
	type jsonEvent struct {
		Timestamp string                 `json:"ts"`
		EventType string                 `json:"event"`
		Data      map[string]interface{} `json:"data,omitempty"`
	}
	return json.Marshal(jsonEvent{
		Timestamp: e.Timestamp.Format(time.RFC3339Nano),
		EventType: e.EventType,
		Data:      e.Data,
	})
}
