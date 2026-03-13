package metrics

import "log"

// Threshold defines a single alert rule: if the named field in an event of
// the given metric type exceeds MaxValue, an alert.threshold event is logged
// with the given severity.
type Threshold struct {
	Metric   string
	Field    string
	MaxValue float64
	Severity string // "warning" or "critical"
}

// AlertChecker evaluates incoming events against a set of thresholds and
// writes alert.threshold events to the EventLogger when a breach is detected.
type AlertChecker struct {
	logger     *EventLogger
	thresholds []Threshold
}

// NewAlertChecker creates an AlertChecker with no thresholds configured.
func NewAlertChecker(logger *EventLogger) *AlertChecker {
	return &AlertChecker{logger: logger}
}

// NewAlertCheckerWithDefaults creates an AlertChecker pre-loaded with
// sensible production thresholds for DB queries, API requests, WS streams,
// heap memory, and goroutine counts.
func NewAlertCheckerWithDefaults(logger *EventLogger) *AlertChecker {
	ac := NewAlertChecker(logger)
	ac.thresholds = []Threshold{
		{Metric: EventDBQuery, Field: "duration_ms", MaxValue: 100, Severity: "warning"},
		{Metric: EventDBQuery, Field: "duration_ms", MaxValue: 500, Severity: "critical"},
		{Metric: EventAPIRequest, Field: "duration_ms", MaxValue: 500, Severity: "warning"},
		{Metric: EventAPIRequest, Field: "duration_ms", MaxValue: 2000, Severity: "critical"},
		{Metric: EventWSStreamEnd, Field: "duration_ms", MaxValue: 300000, Severity: "critical"},
		{Metric: EventSystemSample, Field: "heap_mb", MaxValue: 256, Severity: "warning"},
		{Metric: EventSystemSample, Field: "goroutines", MaxValue: 100, Severity: "warning"},
	}
	return ac
}

// AddThreshold appends a new threshold rule to the checker.
func (ac *AlertChecker) AddThreshold(t Threshold) {
	ac.thresholds = append(ac.thresholds, t)
}

// Check evaluates all thresholds against the given event type and data.
// For each threshold that is breached it logs an alert.threshold event.
// It is safe to call from outside the EventLogger mutex.
func (ac *AlertChecker) Check(eventType string, data map[string]interface{}) {
	for _, t := range ac.thresholds {
		if t.Metric != eventType {
			continue
		}
		value := getFloatField(data, t.Field)
		if value <= t.MaxValue {
			continue
		}
		alertData := map[string]interface{}{
			"metric":    t.Metric,
			"field":     t.Field,
			"value":     value,
			"threshold": t.MaxValue,
			"severity":  t.Severity,
		}
		for _, key := range []string{"method", "path", "session_id"} {
			if v, ok := data[key]; ok {
				alertData[key] = v
			}
		}
		if err := ac.logger.Log(EventAlertThreshold, alertData); err != nil {
			log.Printf("metrics: log alert: %v", err)
		}
	}
}
