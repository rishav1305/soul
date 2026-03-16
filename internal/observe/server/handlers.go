package server

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rishav1305/soul-v2/internal/chat/metrics"
)

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	agg := s.aggregator(r)

	status, err := agg.Status()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	cost, err := agg.Cost()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	alerts, err := agg.Alerts()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": status,
		"cost":   cost,
		"alerts": alerts,
	})
}

func (s *Server) handleLatency(w http.ResponseWriter, r *http.Request) {
	agg := s.aggregator(r)
	report, err := agg.Latency()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	agg := s.aggregator(r)
	report, err := agg.Alerts()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleDB(w http.ResponseWriter, r *http.Request) {
	agg := s.aggregator(r)
	report, err := agg.DB()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleRequests(w http.ResponseWriter, r *http.Request) {
	agg := s.aggregator(r)
	report, err := agg.Requests()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleFrontend(w http.ResponseWriter, r *http.Request) {
	agg := s.aggregator(r)
	report, err := agg.Frontend()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleUsage(w http.ResponseWriter, r *http.Request) {
	agg := s.aggregator(r)
	report, err := agg.Usage()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleQuality(w http.ResponseWriter, r *http.Request) {
	agg := s.aggregator(r)
	report, err := agg.Quality()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleLayers(w http.ResponseWriter, r *http.Request) {
	agg := s.aggregator(r)
	report, err := agg.Layers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleSystem(w http.ResponseWriter, r *http.Request) {
	agg := s.aggregator(r)
	status, err := agg.Status()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"uptime":         status.Uptime,
		"total_sessions": status.TotalSessions,
		"total_messages": status.TotalMessages,
		"active_streams": status.ActiveStreams,
		"total_errors":   status.TotalErrors,
		"last_event":     status.LastEvent,
		"server_uptime":  time.Since(s.startTime).Round(time.Second).String(),
	})
}

func (s *Server) handleTail(w http.ResponseWriter, r *http.Request) {
	product := r.URL.Query().Get("product")
	typePrefix := r.URL.Query().Get("type")

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 500 {
		limit = 500
	}

	events, err := metrics.ReadAllProducts(s.dataDir, product)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Filter by type prefix if specified.
	if typePrefix != "" {
		filtered := make([]metrics.Event, 0, len(events))
		for _, ev := range events {
			if strings.HasPrefix(ev.EventType, typePrefix) {
				filtered = append(filtered, ev)
			}
		}
		events = filtered
	}

	// Return newest-first: reverse and limit.
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.After(events[j].Timestamp)
	})
	if len(events) > limit {
		events = events[:limit]
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"events": events,
		"count":  len(events),
	})
}

// pillarConstraint represents a single constraint within a pillar.
type pillarConstraint struct {
	Name        string `json:"name"`
	Target      string `json:"target"`
	Enforcement string `json:"enforcement"`
	Status      string `json:"status"` // pass, warn, fail, static
	Value       string `json:"value"`
}

// pillarResult represents one of the 6 architectural pillars.
type pillarResult struct {
	Name        string             `json:"name"`
	Constraints []pillarConstraint `json:"constraints"`
	Pass        int                `json:"pass"`
	Warn        int                `json:"warn"`
	Fail        int                `json:"fail"`
	Static      int                `json:"static"`
}

func (s *Server) handlePillars(w http.ResponseWriter, r *http.Request) {
	agg := s.aggregator(r)

	latency, _ := agg.Latency()
	db, _ := agg.DB()
	requests, _ := agg.Requests()
	frontend, _ := agg.Frontend()
	usage, _ := agg.Usage()
	connHealth, _ := agg.ConnectionHealthReport()

	pillars := []pillarResult{
		buildPerformantPillar(latency, db, requests),
		buildRobustPillar(frontend),
		buildResilientPillar(connHealth),
		buildSecurePillar(),
		buildSovereignPillar(),
		buildTransparentPillar(usage, connHealth),
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"pillars": pillars,
	})
}

func buildPerformantPillar(latency *metrics.LatencyReport, db *metrics.DBReport, requests *metrics.RequestsReport) pillarResult {
	p := pillarResult{Name: "performant"}

	// First-token P50 vs 200ms threshold.
	ftStatus := "pass"
	ftValue := "no data"
	if latency != nil && latency.SampleCount > 0 {
		ftMs := latency.FirstTokenP50.Milliseconds()
		ftValue = strconv.FormatInt(ftMs, 10) + "ms"
		if ftMs > 200 {
			ftStatus = "fail"
		} else if ftMs > 150 {
			ftStatus = "warn"
		}
	}
	p.Constraints = append(p.Constraints, pillarConstraint{
		Name:        "first-token-p50",
		Target:      "<200ms",
		Enforcement: "runtime metric",
		Status:      ftStatus,
		Value:       ftValue,
	})

	// DB P50 performance.
	dbStatus := "pass"
	dbValue := "no data"
	if db != nil && len(db.Methods) > 0 {
		var maxP50 int64
		for _, m := range db.Methods {
			if ms := m.P50.Milliseconds(); ms > maxP50 {
				maxP50 = ms
			}
		}
		dbValue = strconv.FormatInt(maxP50, 10) + "ms"
		if maxP50 > 100 {
			dbStatus = "fail"
		} else if maxP50 > 50 {
			dbStatus = "warn"
		}
	}
	p.Constraints = append(p.Constraints, pillarConstraint{
		Name:        "db-query-p50",
		Target:      "<100ms",
		Enforcement: "runtime metric",
		Status:      dbStatus,
		Value:       dbValue,
	})

	// HTTP request P50 performance.
	httpStatus := "pass"
	httpValue := "no data"
	if requests != nil && len(requests.Paths) > 0 {
		var maxP50 int64
		for _, ps := range requests.Paths {
			if ms := ps.P50.Milliseconds(); ms > maxP50 {
				maxP50 = ms
			}
		}
		httpValue = strconv.FormatInt(maxP50, 10) + "ms"
		if maxP50 > 500 {
			httpStatus = "fail"
		} else if maxP50 > 200 {
			httpStatus = "warn"
		}
	}
	p.Constraints = append(p.Constraints, pillarConstraint{
		Name:        "http-request-p50",
		Target:      "<500ms",
		Enforcement: "runtime metric",
		Status:      httpStatus,
		Value:       httpValue,
	})

	countStatuses(&p)
	return p
}

func buildRobustPillar(frontend *metrics.FrontendReport) pillarResult {
	p := pillarResult{Name: "robust"}

	// Frontend error count.
	feStatus := "pass"
	feValue := "0"
	if frontend != nil {
		feValue = strconv.Itoa(frontend.Errors)
		if frontend.Errors > 10 {
			feStatus = "fail"
		} else if frontend.Errors > 0 {
			feStatus = "warn"
		}
	}
	p.Constraints = append(p.Constraints, pillarConstraint{
		Name:        "frontend-errors",
		Target:      "0",
		Enforcement: "runtime metric",
		Status:      feStatus,
		Value:       feValue,
	})

	// Static constraints.
	p.Constraints = append(p.Constraints,
		pillarConstraint{Name: "error-boundaries", Target: "all pages", Enforcement: "enforced at build", Status: "static", Value: "React ErrorBoundary"},
		pillarConstraint{Name: "graceful-degradation", Target: "all features", Enforcement: "enforced at build", Status: "static", Value: "fallback UI patterns"},
	)

	countStatuses(&p)
	return p
}

func buildResilientPillar(ch *metrics.ConnectionHealthReport) pillarResult {
	p := pillarResult{Name: "resilient"}

	// Live: chat drop rate < 0.5%
	dropRate := 0.0
	dropValue := "no data"
	if ch != nil && ch.TotalConnects > 0 {
		dropRate = ch.DropRate()
		dropValue = strconv.FormatFloat(dropRate*100, 'f', 3, 64) + "%"
	}
	dropStatus := "pass"
	if dropRate > 0.005 {
		dropStatus = "fail"
	} else if dropRate > 0.002 {
		dropStatus = "warn"
	}
	p.Constraints = append(p.Constraints, pillarConstraint{
		Name:        "chat-drop-rate",
		Target:      "< 0.5% sessions/hour",
		Enforcement: "runtime metric",
		Status:      dropStatus,
		Value:       dropValue,
	})

	// Live: reconnect success rate > 95%
	reconnectRate := 1.0
	reconnectValue := "no data"
	if ch != nil && (ch.ReconnectSuccesses+ch.ReconnectFailures) > 0 {
		reconnectRate = ch.ReconnectSuccessRate()
		reconnectValue = strconv.FormatFloat(reconnectRate*100, 'f', 1, 64) + "%"
	}
	rateStatus := "pass"
	if reconnectRate < 0.95 {
		rateStatus = "fail"
	} else if reconnectRate < 0.98 {
		rateStatus = "warn"
	}
	p.Constraints = append(p.Constraints, pillarConstraint{
		Name:        "reconnect-success-rate",
		Target:      "> 95%",
		Enforcement: "runtime metric",
		Status:      rateStatus,
		Value:       reconnectValue,
	})

	// Static: graceful shutdown (kept)
	p.Constraints = append(p.Constraints, pillarConstraint{
		Name:        "graceful-shutdown",
		Target:      "SIGTERM handler on all servers",
		Enforcement: "enforced at build",
		Status:      "static",
		Value:       "SIGTERM handler",
	})

	countStatuses(&p)
	return p
}

func buildSecurePillar() pillarResult {
	p := pillarResult{Name: "secure"}
	p.Constraints = []pillarConstraint{
		{Name: "csp-headers", Target: "all responses", Enforcement: "enforced at build", Status: "static", Value: "strict CSP policy"},
		{Name: "sql-injection", Target: "all queries", Enforcement: "enforced at build", Status: "static", Value: "parameterized queries"},
		{Name: "no-hardcoded-secrets", Target: "codebase", Enforcement: "enforced at build", Status: "static", Value: "env vars + Vaultwarden"},
		{Name: "oauth-token-security", Target: "credentials", Enforcement: "enforced at build", Status: "static", Value: "0600 perms, never logged"},
	}
	countStatuses(&p)
	return p
}

func buildSovereignPillar() pillarResult {
	p := pillarResult{Name: "sovereign"}
	p.Constraints = []pillarConstraint{
		{Name: "local-data", Target: "all data", Enforcement: "enforced at build", Status: "static", Value: "SQLite on disk"},
		{Name: "self-hosted", Target: "all services", Enforcement: "enforced at build", Status: "static", Value: "Gitea + Vaultwarden"},
		{Name: "no-cloud-deps", Target: "runtime", Enforcement: "enforced at build", Status: "static", Value: "local-first architecture"},
	}
	countStatuses(&p)
	return p
}

func buildTransparentPillar(usage *metrics.UsageReport, ch *metrics.ConnectionHealthReport) pillarResult {
	p := pillarResult{Name: "transparent"}

	// Event tracking coverage.
	evStatus := "pass"
	evValue := "no data"
	if usage != nil {
		evValue = strconv.Itoa(usage.TotalEvents) + " events"
		if usage.TotalEvents == 0 {
			evStatus = "warn"
		}
	}
	p.Constraints = append(p.Constraints, pillarConstraint{
		Name:        "event-tracking",
		Target:      ">0 events",
		Enforcement: "runtime metric",
		Status:      evStatus,
		Value:       evValue,
	})

	// Usage tracking.
	usageStatus := "pass"
	usageValue := "no data"
	if usage != nil {
		actionCount := len(usage.Actions)
		usageValue = strconv.Itoa(actionCount) + " action types"
		if actionCount == 0 {
			usageStatus = "warn"
		}
	}
	p.Constraints = append(p.Constraints, pillarConstraint{
		Name:        "usage-tracking",
		Target:      ">0 action types",
		Enforcement: "runtime metric",
		Status:      usageStatus,
		Value:       usageValue,
	})

	// Auth event coverage
	authEventCount := 0
	authStatus := "warn"
	if ch != nil {
		authEventCount = ch.AuthFailures + ch.AuthSuccesses
		if authEventCount > 0 {
			authStatus = "pass"
		}
	}
	p.Constraints = append(p.Constraints, pillarConstraint{
		Name:        "auth-event-coverage",
		Target:      "> 0 auth events tracked",
		Enforcement: "runtime metric",
		Status:      authStatus,
		Value:       strconv.Itoa(authEventCount) + " events",
	})

	// WS lifecycle coverage
	wsEventCount := 0
	wsStatus := "warn"
	if ch != nil {
		wsEventCount = ch.TotalConnects + ch.TotalDisconnects
		if wsEventCount > 0 {
			wsStatus = "pass"
		}
	}
	p.Constraints = append(p.Constraints, pillarConstraint{
		Name:        "ws-lifecycle-coverage",
		Target:      "> 0 WS lifecycle events tracked",
		Enforcement: "runtime metric",
		Status:      wsStatus,
		Value:       strconv.Itoa(wsEventCount) + " events",
	})

	// Static constraints.
	p.Constraints = append(p.Constraints,
		pillarConstraint{Name: "metrics-pipeline", Target: "all servers", Enforcement: "enforced at build", Status: "static", Value: "JSONL event logging"},
		pillarConstraint{Name: "cli-reports", Target: "operators", Enforcement: "enforced at build", Status: "static", Value: "soul-chat metrics *"},
	)

	countStatuses(&p)
	return p
}

func countStatuses(p *pillarResult) {
	for _, c := range p.Constraints {
		switch c.Status {
		case "pass":
			p.Pass++
		case "warn":
			p.Warn++
		case "fail":
			p.Fail++
		case "static":
			p.Static++
		}
	}
}
