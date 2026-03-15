package store

import (
	"fmt"
	"time"
)

// AggregateStats holds counts grouped by type, source, and stage.
type AggregateStats struct {
	ByType   map[string]int `json:"byType"`
	BySource map[string]int `json:"bySource"`
	ByStage  map[string]int `json:"byStage"`
	Active   int            `json:"active"`
	Closed   int            `json:"closed"`
	Stale    int            `json:"stale"`
}

// FunnelStep represents a stage in a conversion funnel.
type FunnelStep struct {
	Stage string `json:"stage"`
	Count int    `json:"count"`
}

// TypeFunnel holds conversion metrics for a single lead type.
type TypeFunnel struct {
	Type           string       `json:"type"`
	Steps          []FunnelStep `json:"steps"`
	WinRate        float64      `json:"winRate"`
	AvgDaysToClose float64      `json:"avgDaysToClose"`
}

// ConversionMetrics holds per-type funnel data.
type ConversionMetrics struct {
	Funnels []TypeFunnel `json:"funnels"`
}

// ActionableInsights holds leads requiring attention.
type ActionableInsights struct {
	StaleLeads    []Lead `json:"staleLeads"`
	FollowUpsDue  []Lead `json:"followUpsDue"`
	PipelineGaps  []string `json:"pipelineGaps"`
}

// Analytics combines all three layers of pipeline analytics.
type Analytics struct {
	Stats      AggregateStats     `json:"stats"`
	Conversion ConversionMetrics  `json:"conversion"`
	Insights   ActionableInsights `json:"insights"`
}

// GetAnalytics computes three-layer analytics, optionally filtered by type.
func (s *Store) GetAnalytics(typeFilter string) (*Analytics, error) {
	stats, err := s.getAggregateStats(typeFilter)
	if err != nil {
		return nil, err
	}
	conversion, err := s.getConversionMetrics(typeFilter)
	if err != nil {
		return nil, err
	}
	insights, err := s.getActionableInsights(typeFilter)
	if err != nil {
		return nil, err
	}
	return &Analytics{
		Stats:      *stats,
		Conversion: *conversion,
		Insights:   *insights,
	}, nil
}

func (s *Store) getAggregateStats(typeFilter string) (*AggregateStats, error) {
	stats := &AggregateStats{
		ByType:   make(map[string]int),
		BySource: make(map[string]int),
		ByStage:  make(map[string]int),
	}

	whereClause := ""
	var args []interface{}
	if typeFilter != "" {
		whereClause = " WHERE type = ?"
		args = append(args, typeFilter)
	}

	// By type.
	rows, err := s.db.Query("SELECT type, COUNT(*) FROM leads"+whereClause+" GROUP BY type", args...)
	if err != nil {
		return nil, fmt.Errorf("scout: analytics by type: %w", err)
	}
	for rows.Next() {
		var k string
		var c int
		if err := rows.Scan(&k, &c); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scout: scan by type: %w", err)
		}
		stats.ByType[k] = c
	}
	rows.Close()

	// By source.
	rows, err = s.db.Query("SELECT source, COUNT(*) FROM leads"+whereClause+" GROUP BY source", args...)
	if err != nil {
		return nil, fmt.Errorf("scout: analytics by source: %w", err)
	}
	for rows.Next() {
		var k string
		var c int
		if err := rows.Scan(&k, &c); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scout: scan by source: %w", err)
		}
		stats.BySource[k] = c
	}
	rows.Close()

	// By stage.
	rows, err = s.db.Query("SELECT stage, COUNT(*) FROM leads"+whereClause+" GROUP BY stage", args...)
	if err != nil {
		return nil, fmt.Errorf("scout: analytics by stage: %w", err)
	}
	for rows.Next() {
		var k string
		var c int
		if err := rows.Scan(&k, &c); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scout: scan by stage: %w", err)
		}
		stats.ByStage[k] = c
	}
	rows.Close()

	// Active / closed / stale counts.
	var active, closed, stale int
	staleThreshold := time.Now().UTC().Add(-7 * 24 * time.Hour).Format(time.RFC3339)

	err = s.db.QueryRow("SELECT COUNT(*) FROM leads"+whereClause+" AND closed_at = ''", args...).Scan(&active)
	if err != nil {
		// If no WHERE clause, we need different SQL.
		if typeFilter == "" {
			err = s.db.QueryRow("SELECT COUNT(*) FROM leads WHERE closed_at = ''").Scan(&active)
		}
		if err != nil {
			return nil, fmt.Errorf("scout: count active: %w", err)
		}
	}

	if typeFilter != "" {
		err = s.db.QueryRow("SELECT COUNT(*) FROM leads WHERE type = ? AND closed_at != ''", typeFilter).Scan(&closed)
	} else {
		err = s.db.QueryRow("SELECT COUNT(*) FROM leads WHERE closed_at != ''").Scan(&closed)
	}
	if err != nil {
		return nil, fmt.Errorf("scout: count closed: %w", err)
	}

	if typeFilter != "" {
		err = s.db.QueryRow("SELECT COUNT(*) FROM leads WHERE type = ? AND closed_at = '' AND updated_at < ?", typeFilter, staleThreshold).Scan(&stale)
	} else {
		err = s.db.QueryRow("SELECT COUNT(*) FROM leads WHERE closed_at = '' AND updated_at < ?", staleThreshold).Scan(&stale)
	}
	if err != nil {
		return nil, fmt.Errorf("scout: count stale: %w", err)
	}

	stats.Active = active
	stats.Closed = closed
	stats.Stale = stale

	return stats, nil
}

func (s *Store) getConversionMetrics(typeFilter string) (*ConversionMetrics, error) {
	cm := &ConversionMetrics{}

	// Get distinct types.
	var typeRows []string
	if typeFilter != "" {
		typeRows = []string{typeFilter}
	} else {
		rows, err := s.db.Query("SELECT DISTINCT type FROM leads")
		if err != nil {
			return nil, fmt.Errorf("scout: conversion types: %w", err)
		}
		for rows.Next() {
			var t string
			if err := rows.Scan(&t); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scout: scan type: %w", err)
			}
			typeRows = append(typeRows, t)
		}
		rows.Close()
	}

	for _, lt := range typeRows {
		funnel := TypeFunnel{Type: lt}

		// Stage counts for this type.
		rows, err := s.db.Query("SELECT stage, COUNT(*) FROM leads WHERE type = ? GROUP BY stage", lt)
		if err != nil {
			return nil, fmt.Errorf("scout: funnel stages: %w", err)
		}
		var total, closed int
		for rows.Next() {
			var stage string
			var count int
			if err := rows.Scan(&stage, &count); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scout: scan funnel: %w", err)
			}
			funnel.Steps = append(funnel.Steps, FunnelStep{Stage: stage, Count: count})
			total += count
		}
		rows.Close()

		// Win rate: leads with non-empty closed_at / total.
		err = s.db.QueryRow("SELECT COUNT(*) FROM leads WHERE type = ? AND closed_at != ''", lt).Scan(&closed)
		if err != nil {
			return nil, fmt.Errorf("scout: win rate: %w", err)
		}
		if total > 0 {
			funnel.WinRate = float64(closed) / float64(total)
		}

		// Average days to close.
		rows2, err := s.db.Query("SELECT created_at, closed_at FROM leads WHERE type = ? AND closed_at != ''", lt)
		if err != nil {
			return nil, fmt.Errorf("scout: avg close: %w", err)
		}
		var totalDays float64
		var closedCount int
		for rows2.Next() {
			var createdStr, closedStr string
			if err := rows2.Scan(&createdStr, &closedStr); err != nil {
				rows2.Close()
				return nil, fmt.Errorf("scout: scan close times: %w", err)
			}
			created, err1 := time.Parse(time.RFC3339, createdStr)
			closedAt, err2 := time.Parse(time.RFC3339, closedStr)
			if err1 == nil && err2 == nil {
				totalDays += closedAt.Sub(created).Hours() / 24
				closedCount++
			}
		}
		rows2.Close()
		if closedCount > 0 {
			funnel.AvgDaysToClose = totalDays / float64(closedCount)
		}

		cm.Funnels = append(cm.Funnels, funnel)
	}

	return cm, nil
}

func (s *Store) getActionableInsights(typeFilter string) (*ActionableInsights, error) {
	insights := &ActionableInsights{}
	staleThreshold := time.Now().UTC().Add(-7 * 24 * time.Hour).Format(time.RFC3339)
	todayStr := time.Now().UTC().Format("2006-01-02")

	// Stale leads: active, no update in 7+ days.
	staleQuery := "SELECT " + leadColumns + " FROM leads WHERE closed_at = '' AND updated_at < ?"
	staleArgs := []interface{}{staleThreshold}
	if typeFilter != "" {
		staleQuery += " AND type = ?"
		staleArgs = append(staleArgs, typeFilter)
	}
	staleQuery += " ORDER BY updated_at ASC"

	rows, err := s.db.Query(staleQuery, staleArgs...)
	if err != nil {
		return nil, fmt.Errorf("scout: stale leads: %w", err)
	}
	for rows.Next() {
		l, err := scanLead(rows)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("scout: scan stale: %w", err)
		}
		insights.StaleLeads = append(insights.StaleLeads, *l)
	}
	rows.Close()

	// Follow-ups due: next_date <= today.
	followQuery := "SELECT " + leadColumns + " FROM leads WHERE closed_at = '' AND next_date != '' AND next_date <= ?"
	followArgs := []interface{}{todayStr}
	if typeFilter != "" {
		followQuery += " AND type = ?"
		followArgs = append(followArgs, typeFilter)
	}
	followQuery += " ORDER BY next_date ASC"

	rows, err = s.db.Query(followQuery, followArgs...)
	if err != nil {
		return nil, fmt.Errorf("scout: follow ups: %w", err)
	}
	for rows.Next() {
		l, err := scanLead(rows)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("scout: scan follow up: %w", err)
		}
		insights.FollowUpsDue = append(insights.FollowUpsDue, *l)
	}
	rows.Close()

	// Pipeline gaps: types with zero active leads.
	knownTypes := []string{"job", "freelance", "contract", "consulting", "product-dev"}
	for _, kt := range knownTypes {
		if typeFilter != "" && typeFilter != kt {
			continue
		}
		var count int
		err := s.db.QueryRow("SELECT COUNT(*) FROM leads WHERE type = ? AND closed_at = ''", kt).Scan(&count)
		if err != nil {
			return nil, fmt.Errorf("scout: pipeline gap check: %w", err)
		}
		if count == 0 {
			insights.PipelineGaps = append(insights.PipelineGaps, kt)
		}
	}

	return insights, nil
}
