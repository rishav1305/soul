package metrics

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// Aggregator reads JSONL events and computes derived metrics reports.
type Aggregator struct {
	dataDir string
	product string // optional product filter; empty = all products
}

// StatusReport provides a system overview: uptime, sessions, messages, errors.
type StatusReport struct {
	Uptime        string
	TotalSessions int
	TotalMessages int
	ActiveStreams  int
	TotalErrors   int
	LastEvent     time.Time
}

// QualityEntry represents a single quality rating from an override.quality event.
type QualityEntry struct {
	Step   string
	Rating int
	Notes  string
}

// QualityReport provides error taxonomy counts and quality ratings.
type QualityReport struct {
	ErrorCounts    map[string]int
	QualityRatings []QualityEntry
	TotalErrors    int
	FalsePositives int
}

// GateResult holds pass/fail/retry counts for a single gate name.
type GateResult struct {
	Gate    string
	Pass    int
	Fail    int
	Retry   int
	Total   int
}

// LayersReport provides gate pass/fail/retry rates per layer.
type LayersReport struct {
	GateResults []GateResult
}

// CostReport provides token usage and estimated cost.
type CostReport struct {
	InputTokens   int
	OutputTokens  int
	TotalRequests int
	EstimatedCost float64
}

// LatencyReport provides streaming performance percentiles.
type LatencyReport struct {
	FirstTokenP50 time.Duration
	FirstTokenP95 time.Duration
	FirstTokenP99 time.Duration
	StreamP50     time.Duration
	StreamP95     time.Duration
	StreamP99     time.Duration
	SampleCount   int
}

// AlertEntry represents a single threshold breach.
type AlertEntry struct {
	Timestamp time.Time
	Metric    string
	Field     string
	Value     float64
	Threshold float64
	Severity  string
}

type AlertsReport struct {
	Alerts []AlertEntry
}

type MethodStats struct {
	Method string
	Count  int
	P50    time.Duration
	P95    time.Duration
	P99    time.Duration
}

type SlowQuery struct {
	Timestamp  time.Time
	Method     string
	DurationMs float64
	SessionID  string
}

type DBReport struct {
	Methods     map[string]*MethodStats
	SlowQueries []SlowQuery
}

type PathStats struct {
	Path  string
	Count int
	P50   time.Duration
	P95   time.Duration
	P99   time.Duration
}

type RequestsReport struct {
	Paths       map[string]*PathStats
	StatusCodes map[int]int
}

type RenderEntry struct {
	Component  string
	DurationMs float64
}

type FrontendReport struct {
	Errors      int
	TopErrors   map[string]int
	SlowRenders []RenderEntry
}

// UsageReport provides page view counts and feature action counts.
type UsageReport struct {
	PageViews   map[string]int
	Actions     map[string]int
	TotalEvents int
}

// Cost estimation rates (rough, Sonnet pricing).
const (
	inputCostPerMToken  = 3.0  // $3 per million input tokens
	outputCostPerMToken = 15.0 // $15 per million output tokens
)

// NewAggregator creates a new Aggregator for the given data directory.
// It reads events from all product files (no product filter).
func NewAggregator(dataDir string) *Aggregator {
	return &Aggregator{dataDir: dataDir}
}

// NewAggregatorForProduct creates an Aggregator that filters events to a
// specific product. If product is empty, it behaves like NewAggregator.
func NewAggregatorForProduct(dataDir string, product string) *Aggregator {
	return &Aggregator{dataDir: dataDir, product: product}
}

// readProductEvents reads events from product-aware metric files, optionally
// filtering by type prefix. When the aggregator has a product set, only that
// product's files are read; otherwise all product files are included.
func (a *Aggregator) readProductEvents(typePrefix string) ([]Event, error) {
	events, err := ReadAllProducts(a.dataDir, a.product)
	if err != nil {
		return nil, fmt.Errorf("read events: %w", err)
	}
	if typePrefix == "" {
		return events, nil
	}
	var filtered []Event
	for _, ev := range events {
		if strings.HasPrefix(ev.EventType, typePrefix) {
			filtered = append(filtered, ev)
		}
	}
	if filtered == nil {
		filtered = []Event{}
	}
	return filtered, nil
}

// Status reads all events and computes a system overview.
func (a *Aggregator) Status() (*StatusReport, error) {
	events, err := a.readProductEvents("")
	if err != nil {
		return nil, fmt.Errorf("read events: %w", err)
	}

	report := &StatusReport{
		Uptime: "unknown",
	}

	if len(events) == 0 {
		return report, nil
	}

	report.LastEvent = events[len(events)-1].Timestamp

	sessions := make(map[string]bool)
	activeStreams := make(map[string]bool)
	var startTime time.Time

	for _, ev := range events {
		switch ev.EventType {
		case EventSystemStart:
			startTime = ev.Timestamp
		case EventWSConnect:
			if clientID := getStringField(ev.Data, "client_id"); clientID != "" {
				sessions[clientID] = true
			} else if client := getStringField(ev.Data, "client"); client != "" {
				sessions[client] = true
			}
			report.TotalSessions++
		case EventWSStreamStart:
			if id := getStringField(ev.Data, "stream_id"); id != "" {
				activeStreams[id] = true
			}
		case EventWSStreamEnd:
			if id := getStringField(ev.Data, "stream_id"); id != "" {
				delete(activeStreams, id)
			}
		case EventAPIRequest:
			report.TotalMessages++
		case EventAPIError, EventOverrideError:
			report.TotalErrors++
		}
	}

	report.ActiveStreams = len(activeStreams)

	if !startTime.IsZero() {
		report.Uptime = time.Since(startTime).Truncate(time.Second).String()
	}

	return report, nil
}

// Quality reads override events and computes error taxonomy counts and quality ratings.
func (a *Aggregator) Quality() (*QualityReport, error) {
	events, err := a.readProductEvents("override")
	if err != nil {
		return nil, fmt.Errorf("read events: %w", err)
	}

	report := &QualityReport{
		ErrorCounts: make(map[string]int),
	}

	for _, ev := range events {
		switch ev.EventType {
		case EventOverrideError:
			errType := getStringField(ev.Data, "type")
			if errType == ErrorFalsePositive {
				report.FalsePositives++
			} else {
				report.TotalErrors++
				if errType != "" {
					report.ErrorCounts[errType]++
				} else {
					report.ErrorCounts["unknown"]++
				}
			}
		case EventOverrideQuality:
			entry := QualityEntry{
				Step:   getStringField(ev.Data, "step"),
				Notes:  getStringField(ev.Data, "notes"),
				Rating: getIntField(ev.Data, "rating"),
			}
			report.QualityRatings = append(report.QualityRatings, entry)
		}
	}

	return report, nil
}

// Layers reads gate events and computes pass/fail/retry counts per gate.
func (a *Aggregator) Layers() (*LayersReport, error) {
	events, err := a.readProductEvents("gate")
	if err != nil {
		return nil, fmt.Errorf("read events: %w", err)
	}

	gateMap := make(map[string]*GateResult)

	for _, ev := range events {
		gateName := getStringField(ev.Data, "gate")
		if gateName == "" {
			gateName = getStringField(ev.Data, "name")
		}
		if gateName == "" {
			gateName = "unknown"
		}

		result, ok := gateMap[gateName]
		if !ok {
			result = &GateResult{Gate: gateName}
			gateMap[gateName] = result
		}

		switch ev.EventType {
		case EventGatePass:
			result.Pass++
			result.Total++
		case EventGateFail:
			result.Fail++
			result.Total++
		case EventGateRetry:
			result.Retry++
			result.Total++
		case EventGateRun:
			result.Total++
		}
	}

	report := &LayersReport{}
	for _, r := range gateMap {
		report.GateResults = append(report.GateResults, *r)
	}

	// Sort by gate name for deterministic output.
	sort.Slice(report.GateResults, func(i, j int) bool {
		return report.GateResults[i].Gate < report.GateResults[j].Gate
	})

	return report, nil
}

// Cost reads api.request events and computes token usage and estimated cost.
func (a *Aggregator) Cost() (*CostReport, error) {
	events, err := a.readProductEvents("api")
	if err != nil {
		return nil, fmt.Errorf("read events: %w", err)
	}

	report := &CostReport{}

	for _, ev := range events {
		if ev.EventType != EventAPIRequest {
			continue
		}
		report.TotalRequests++
		report.InputTokens += getIntField(ev.Data, "input_tokens")
		report.OutputTokens += getIntField(ev.Data, "output_tokens")
	}

	report.EstimatedCost = estimateCost(report.InputTokens, report.OutputTokens)

	return report, nil
}

// Latency reads ws.stream.token and ws.stream.end events and computes percentiles.
func (a *Aggregator) Latency() (*LatencyReport, error) {
	events, err := a.readProductEvents("ws.stream")
	if err != nil {
		return nil, fmt.Errorf("read events: %w", err)
	}

	var firstTokenMs []float64
	var streamMs []float64

	for _, ev := range events {
		switch ev.EventType {
		case EventWSStreamToken:
			if v := getFloatField(ev.Data, "first_token_ms"); v > 0 {
				firstTokenMs = append(firstTokenMs, v)
			}
		case EventWSStreamEnd:
			if v := getFloatField(ev.Data, "duration_ms"); v > 0 {
				streamMs = append(streamMs, v)
			}
		}
	}

	report := &LatencyReport{
		SampleCount: len(firstTokenMs),
	}

	if len(firstTokenMs) > 0 {
		sort.Float64s(firstTokenMs)
		report.FirstTokenP50 = msToDuration(percentile(firstTokenMs, 50))
		report.FirstTokenP95 = msToDuration(percentile(firstTokenMs, 95))
		report.FirstTokenP99 = msToDuration(percentile(firstTokenMs, 99))
	}

	if len(streamMs) > 0 {
		sort.Float64s(streamMs)
		report.StreamP50 = msToDuration(percentile(streamMs, 50))
		report.StreamP95 = msToDuration(percentile(streamMs, 95))
		report.StreamP99 = msToDuration(percentile(streamMs, 99))
	}

	return report, nil
}

// Alerts reads alert events and returns all threshold breach entries.
func (a *Aggregator) Alerts() (*AlertsReport, error) {
	events, err := a.readProductEvents("alert")
	if err != nil {
		return nil, fmt.Errorf("read events: %w", err)
	}
	report := &AlertsReport{}
	for _, ev := range events {
		if ev.EventType != EventAlertThreshold {
			continue
		}
		report.Alerts = append(report.Alerts, AlertEntry{
			Timestamp: ev.Timestamp,
			Metric:    getStringField(ev.Data, "metric"),
			Field:     getStringField(ev.Data, "field"),
			Value:     getFloatField(ev.Data, "value"),
			Threshold: getFloatField(ev.Data, "threshold"),
			Severity:  getStringField(ev.Data, "severity"),
		})
	}
	return report, nil
}

// DB reads db events and computes per-method performance percentiles and slow queries.
func (a *Aggregator) DB() (*DBReport, error) {
	events, err := a.readProductEvents("db")
	if err != nil {
		return nil, fmt.Errorf("read events: %w", err)
	}
	report := &DBReport{Methods: make(map[string]*MethodStats)}
	methodDurations := make(map[string][]float64)

	for _, ev := range events {
		method := getStringField(ev.Data, "method")
		dur := getFloatField(ev.Data, "duration_ms")
		switch ev.EventType {
		case EventDBQuery:
			methodDurations[method] = append(methodDurations[method], dur)
		case EventDBSlow:
			report.SlowQueries = append(report.SlowQueries, SlowQuery{
				Timestamp:  ev.Timestamp,
				Method:     method,
				DurationMs: dur,
				SessionID:  getStringField(ev.Data, "session_id"),
			})
		}
	}

	for method, durations := range methodDurations {
		sort.Float64s(durations)
		report.Methods[method] = &MethodStats{
			Method: method,
			Count:  len(durations),
			P50:    msToDuration(percentile(durations, 50)),
			P95:    msToDuration(percentile(durations, 95)),
			P99:    msToDuration(percentile(durations, 99)),
		}
	}
	return report, nil
}

// Requests reads api events and computes per-path performance percentiles and status code counts.
func (a *Aggregator) Requests() (*RequestsReport, error) {
	events, err := a.readProductEvents("api")
	if err != nil {
		return nil, fmt.Errorf("read events: %w", err)
	}
	report := &RequestsReport{
		Paths:       make(map[string]*PathStats),
		StatusCodes: make(map[int]int),
	}
	pathDurations := make(map[string][]float64)

	for _, ev := range events {
		if ev.EventType != EventAPIRequest {
			continue
		}
		path := getStringField(ev.Data, "path")
		dur := getFloatField(ev.Data, "duration_ms")
		status := getIntField(ev.Data, "status")

		pathDurations[path] = append(pathDurations[path], dur)
		if status > 0 {
			report.StatusCodes[status]++
		}
	}

	for path, durations := range pathDurations {
		sort.Float64s(durations)
		report.Paths[path] = &PathStats{
			Path:  path,
			Count: len(durations),
			P50:   msToDuration(percentile(durations, 50)),
			P95:   msToDuration(percentile(durations, 95)),
			P99:   msToDuration(percentile(durations, 99)),
		}
	}
	return report, nil
}

// Frontend reads frontend events and computes error counts and slow render entries.
func (a *Aggregator) Frontend() (*FrontendReport, error) {
	events, err := a.readProductEvents("frontend")
	if err != nil {
		return nil, fmt.Errorf("read events: %w", err)
	}
	report := &FrontendReport{TopErrors: make(map[string]int)}

	for _, ev := range events {
		switch ev.EventType {
		case EventFrontendError:
			report.Errors++
			comp := getStringField(ev.Data, "component")
			if comp != "" {
				report.TopErrors[comp]++
			}
		case EventFrontendRender:
			report.SlowRenders = append(report.SlowRenders, RenderEntry{
				Component:  getStringField(ev.Data, "component"),
				DurationMs: getFloatField(ev.Data, "duration_ms"),
			})
		}
	}
	return report, nil
}

// Usage reads frontend.usage events and computes page views and action counts.
func (a *Aggregator) Usage() (*UsageReport, error) {
	events, err := a.readProductEvents("frontend.usage")
	if err != nil {
		return nil, fmt.Errorf("read events: %w", err)
	}
	report := &UsageReport{
		PageViews: make(map[string]int),
		Actions:   make(map[string]int),
	}

	for _, ev := range events {
		report.TotalEvents++
		action := getStringField(ev.Data, "action")
		if action == "" {
			continue
		}
		if action == "page.view" {
			page := getStringField(ev.Data, "page")
			if page != "" {
				report.PageViews[page]++
			}
		} else {
			report.Actions[action]++
		}
	}
	return report, nil
}

// estimateCost calculates rough USD cost based on Sonnet pricing.
func estimateCost(inputTokens, outputTokens int) float64 {
	inputCost := float64(inputTokens) / 1_000_000 * inputCostPerMToken
	outputCost := float64(outputTokens) / 1_000_000 * outputCostPerMToken
	return math.Round((inputCost+outputCost)*10000) / 10000 // 4 decimal places
}

// percentile returns the p-th percentile from a sorted slice of float64 values.
// Uses nearest-rank method. The slice must be sorted in ascending order.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	rank := (p / 100) * float64(len(sorted))
	idx := int(math.Ceil(rank)) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// msToDuration converts milliseconds (float64) to time.Duration.
func msToDuration(ms float64) time.Duration {
	return time.Duration(ms * float64(time.Millisecond))
}

// getStringField safely extracts a string value from the event data map.
func getStringField(data map[string]interface{}, key string) string {
	if data == nil {
		return ""
	}
	v, ok := data[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// getIntField safely extracts an integer value from the event data map.
// JSON numbers are decoded as float64 by encoding/json, so we handle that.
func getIntField(data map[string]interface{}, key string) int {
	if data == nil {
		return 0
	}
	v, ok := data[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	default:
		return 0
	}
}

// getFloatField safely extracts a float64 value from the event data map.
func getFloatField(data map[string]interface{}, key string) float64 {
	if data == nil {
		return 0
	}
	v, ok := data[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	default:
		return 0
	}
}

// ConnectionHealthReport contains connection reliability metrics derived from JSONL events.
type ConnectionHealthReport struct {
	TotalConnects       int
	TotalDisconnects    int
	AbnormalDisconnects int
	AuthFailures        int
	AuthSuccesses       int
	ReconnectSuccesses  int
	ReconnectFailures   int
}

// DropRate returns the fraction of abnormal disconnects vs total connects.
func (r *ConnectionHealthReport) DropRate() float64 {
	if r.TotalConnects == 0 {
		return 0
	}
	return float64(r.AbnormalDisconnects) / float64(r.TotalConnects)
}

// ReconnectSuccessRate returns the fraction of successful reconnects.
func (r *ConnectionHealthReport) ReconnectSuccessRate() float64 {
	total := r.ReconnectSuccesses + r.ReconnectFailures
	if total == 0 {
		return 1.0
	}
	return float64(r.ReconnectSuccesses) / float64(total)
}

// ConnectionHealthReport reads JSONL metrics and computes connection health.
func (a *Aggregator) ConnectionHealthReport() (*ConnectionHealthReport, error) {
	events, err := a.readProductEvents("") // empty prefix = all event types
	if err != nil {
		return nil, err
	}

	report := &ConnectionHealthReport{}
	for _, ev := range events {
		switch ev.EventType {
		case EventWSConnect:
			report.TotalConnects++
		case EventWSClose:
			report.TotalDisconnects++
			if reason, ok := ev.Data["reason_class"].(string); ok {
				if reason != "normal" && reason != "client_nav" {
					report.AbnormalDisconnects++
				}
			}
		case EventAuthFail:
			report.AuthFailures++
		case EventAuthOK:
			report.AuthSuccesses++
		case EventWSReconnectSuccess:
			report.ReconnectSuccesses++
		case EventWSReconnectFail:
			report.ReconnectFailures++
		}
	}
	return report, nil
}
