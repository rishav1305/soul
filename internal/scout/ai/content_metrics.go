package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ContentMetricsResult is the structured output from content metrics analysis.
type ContentMetricsResult struct {
	TotalPosts       int                    `json:"total_posts"`
	TotalImpressions int                    `json:"total_impressions"`
	TotalEngagement  int                    `json:"total_engagement"`
	AvgEngagement    string                 `json:"avg_engagement_rate"`
	TopPerforming    []TopPerformingPost    `json:"top_performing"`
	Analysis         string                 `json:"analysis"`
	Recommendations  []string               `json:"recommendations"`
}

// TopPerformingPost summarizes a high-performing content post.
type TopPerformingPost struct {
	Topic       string `json:"topic"`
	Impressions int    `json:"impressions"`
}

// ContentMetrics aggregates content performance across all published posts
// and sends to Claude for analysis. Pass platform="" for all platforms.
func (s *Service) ContentMetrics(ctx context.Context, platform string) (*ContentMetricsResult, error) {
	posts, err := s.store.ListContentPosts(platform, "published")
	if err != nil {
		return nil, fmt.Errorf("list content posts: %w", err)
	}

	totalPosts := len(posts)
	var totalImpressions, totalLikes, totalComments, totalShares int
	for _, p := range posts {
		totalImpressions += p.Impressions
		totalLikes += p.Likes
		totalComments += p.Comments
		totalShares += p.Shares
	}
	totalEngagement := totalLikes + totalComments + totalShares

	var avgRate string
	if totalImpressions > 0 {
		pct := float64(totalEngagement) / float64(totalImpressions) * 100
		avgRate = fmt.Sprintf("%.2f%%", pct)
	} else {
		avgRate = "0.00%"
	}

	// Top 3 by impressions
	sorted := make([]TopPerformingPost, len(posts))
	for i, p := range posts {
		sorted[i] = TopPerformingPost{Topic: p.Topic, Impressions: p.Impressions}
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Impressions > sorted[j].Impressions
	})
	top := sorted
	if len(top) > 3 {
		top = top[:3]
	}

	// If no posts, return zero metrics without calling Claude
	if totalPosts == 0 {
		return &ContentMetricsResult{
			TotalPosts:       0,
			TotalImpressions: 0,
			TotalEngagement:  0,
			AvgEngagement:    avgRate,
			TopPerforming:    []TopPerformingPost{},
			Analysis:         "No published content to analyze.",
			Recommendations:  []string{"Start publishing content to build metrics."},
		}, nil
	}

	// Build summary for Claude
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Total Posts: %d\n", totalPosts))
	sb.WriteString(fmt.Sprintf("Total Impressions: %d\n", totalImpressions))
	sb.WriteString(fmt.Sprintf("Total Likes: %d\n", totalLikes))
	sb.WriteString(fmt.Sprintf("Total Comments: %d\n", totalComments))
	sb.WriteString(fmt.Sprintf("Total Shares: %d\n", totalShares))
	sb.WriteString(fmt.Sprintf("Avg Engagement Rate: %s\n", avgRate))
	sb.WriteString("\nTop Performing Posts:\n")
	for i, t := range top {
		sb.WriteString(fmt.Sprintf("%d. %s (%d impressions)\n", i+1, t.Topic, t.Impressions))
	}
	sb.WriteString("\nAll Posts:\n")
	for _, p := range posts {
		sb.WriteString(fmt.Sprintf("- %s [%s/%s]: %d impressions, %d likes, %d comments, %d shares\n",
			p.Topic, p.Platform, p.Pillar, p.Impressions, p.Likes, p.Comments, p.Shares))
	}

	system := "You are a content analytics expert. Analyze these content metrics and provide actionable insights on what content performs best and recommendations for improvement. Return ONLY valid JSON: {\"analysis\": \"...\", \"recommendations\": [\"...\"]}"

	text, err := s.sendAndExtractText(ctx, system, sb.String())
	if err != nil {
		return nil, err
	}

	cleaned := strings.TrimSpace(extractJSON(text))
	var aiResult struct {
		Analysis        string   `json:"analysis"`
		Recommendations []string `json:"recommendations"`
	}
	if err := json.Unmarshal([]byte(cleaned), &aiResult); err != nil {
		return nil, fmt.Errorf("parse metrics analysis: %w (raw: %s)", err, text)
	}

	return &ContentMetricsResult{
		TotalPosts:       totalPosts,
		TotalImpressions: totalImpressions,
		TotalEngagement:  totalEngagement,
		AvgEngagement:    avgRate,
		TopPerforming:    top,
		Analysis:         aiResult.Analysis,
		Recommendations:  aiResult.Recommendations,
	}, nil
}
