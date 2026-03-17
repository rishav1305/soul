package sweep

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/rishav1305/soul-v2/internal/scout/store"
)

// SweepResult holds the outcome of a sweep run.
type SweepResult struct {
	NewLeads    int      `json:"newLeads"`
	Duplicates  int      `json:"duplicates"`
	Scored      int      `json:"scored"`
	HighMatches int      `json:"highMatches"`
	Errors      []string `json:"errors"`
}

// Scorer scores a lead by ID. Implemented by ai.Service.ScoreLead.
type Scorer interface {
	ScoreLead(leadID int64) (float64, error)
}

// RunSweep executes a 3-phase sweep: fetch → score → finalize.
func RunSweep(client *TheirStackClient, st *store.Store, cfg *SweepConfig, scorer Scorer) (*SweepResult, error) {
	if client == nil {
		return nil, fmt.Errorf("sweep: TheirStack client is nil")
	}
	if st == nil {
		return nil, fmt.Errorf("sweep: store is nil")
	}
	if cfg == nil {
		cfg = DefaultConfig()
	}
	result := &SweepResult{}

	// Load cursor (empty on first run = fetch all)
	cursor, err := st.GetSyncMeta("theirstack_cursor")
	if err != nil {
		log.Printf("scout: load cursor: %v", err)
	}

	// Phase 1: Fetch all pages
	var newLeadIDs []int64
	var maxDiscoveredAt string
	creditsUsed := 0
	offset := 0
	hadError := false

	for {
		resp, err := client.Search(cfg, cursor, offset)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("page offset=%d: %v", offset, err))
			hadError = true
			break
		}

		for _, job := range resp.Jobs {
			if job.DiscoveredAt > maxDiscoveredAt {
				maxDiscoveredAt = job.DiscoveredAt
			}

			lead := JobToLead(job)
			id, created, err := st.AddLeadIfNotExists(lead)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("insert job %d: %v", job.ID, err))
				continue
			}
			if created {
				result.NewLeads++
				newLeadIDs = append(newLeadIDs, id)
			} else {
				result.Duplicates++
			}
		}

		creditsUsed += len(resp.Jobs)
		if len(resp.Jobs) < cfg.Limit || creditsUsed >= cfg.CreditBudget {
			break
		}
		offset += cfg.Limit
	}

	// Phase 2: Auto-score
	if scorer != nil {
		for _, leadID := range newLeadIDs {
			score, err := scorer.ScoreLead(leadID)
			if err != nil {
				log.Printf("scout: score lead %d: %v", leadID, err)
				continue
			}
			result.Scored++
			if score >= cfg.AutoScoreThreshold {
				result.HighMatches++
			}
		}
	}

	// Phase 3: Finalize — log errors but don't fail the sweep result
	if !hadError && maxDiscoveredAt != "" {
		t, err := time.Parse(time.RFC3339, maxDiscoveredAt)
		if err != nil {
			log.Printf("scout: parse discovered_at %q: %v", maxDiscoveredAt, err)
		} else {
			newCursor := t.Add(1 * time.Second).Format(time.RFC3339)
			if err := st.SetSyncMeta("theirstack_cursor", newCursor); err != nil {
				log.Printf("scout: save cursor: %v", err)
				result.Errors = append(result.Errors, "failed to save cursor: "+err.Error())
			}
		}
	}
	if err := st.SetSyncMeta("sweep_last_run", time.Now().UTC().Format(time.RFC3339)); err != nil {
		log.Printf("scout: save last_run: %v", err)
	}

	digest := BuildDigest(result, st, cfg)
	digestBytes, err := json.Marshal(digest)
	if err != nil {
		log.Printf("scout: marshal digest: %v", err)
	} else {
		if err := st.SetSyncMeta("sweep_last_digest", string(digestBytes)); err != nil {
			log.Printf("scout: save digest: %v", err)
		}
	}

	return result, nil
}

// JobToLead converts a TheirStack Job to a store.Lead.
func JobToLead(job Job) store.Lead {
	pipeline := InferPipeline(job.EmploymentStatuses)
	tsID := int64(job.ID)

	statuses, _ := json.Marshal(job.EmploymentStatuses)
	techs, _ := json.Marshal(job.TechnologySlugs)
	keywords, _ := json.Marshal(job.KeywordSlugs)

	lead := store.Lead{
		Source:             "theirstack",
		Pipeline:           pipeline,
		Stage:              "discovered",
		NextAction:         "review",
		TheirStackID:       &tsID,
		JobTitle:           job.JobTitle,
		URL:                job.URL,
		FinalURL:           job.FinalURL,
		SourceURL:          job.SourceURL,
		DatePosted:         job.DatePosted,
		DiscoveredAt:       job.DiscoveredAt,
		Description:        job.Description,
		NormalizedTitle:    job.NormalizedTitle,
		Location:           job.Location,
		ShortLocation:      job.ShortLocation,
		Country:            job.Country,
		CountryCode:        job.CountryCode,
		Remote:             job.Remote,
		Hybrid:             job.Hybrid,
		SalaryString:       job.SalaryString,
		MinAnnualSalaryUSD: job.MinAnnualSalaryUSD,
		MaxAnnualSalaryUSD: job.MaxAnnualSalaryUSD,
		SalaryCurrency:     job.SalaryCurrency,
		Seniority:          job.Seniority,
		EmploymentStatuses: string(statuses),
		EasyApply:          job.EasyApply,
		TechnologySlugs:    string(techs),
		KeywordSlugs:       string(keywords),
		Company:            job.Company,
		CompanyDomain:      job.CompanyDomain,
	}

	// Extract company info if present
	if job.CompanyObject != nil {
		lead.CompanyIndustry = job.CompanyObject.Industry
		lead.CompanyEmployeeCount = job.CompanyObject.EmployeeCount
		lead.CompanyLinkedInURL = job.CompanyObject.LinkedInURL
		lead.CompanyTotalFundingUSD = job.CompanyObject.TotalFundingUSD
		lead.CompanyFundingStage = job.CompanyObject.FundingStage
		lead.CompanyLogo = job.CompanyObject.Logo
		lead.CompanyCountry = job.CompanyObject.Country
		if lead.CompanyDomain == "" {
			lead.CompanyDomain = job.CompanyObject.Domain
		}
	}

	// Extract first hiring team member
	if len(job.HiringTeam) > 0 {
		lead.HiringManager = job.HiringTeam[0].FullName
		lead.HiringManagerLinkedIn = job.HiringTeam[0].LinkedInURL
	}

	// Build metadata for remaining fields
	meta := map[string]interface{}{}
	if len(job.HiringTeam) > 1 {
		meta["hiring_team"] = job.HiringTeam[1:]
	}
	if len(meta) > 0 {
		metaBytes, _ := json.Marshal(meta)
		lead.Metadata = string(metaBytes)
	}

	return lead
}

// BuildDigest constructs the sweep digest from results and DB state.
func BuildDigest(result *SweepResult, st *store.Store, cfg *SweepConfig) map[string]interface{} {
	lastRun, _ := st.GetSyncMeta("sweep_last_run")
	highLeads, _ := st.ScoredLeads(10)
	var highMatchLeads []map[string]interface{}
	for _, l := range highLeads {
		if l.MatchScore >= cfg.AutoScoreThreshold {
			highMatchLeads = append(highMatchLeads, map[string]interface{}{
				"id": l.ID, "job_title": l.JobTitle, "company": l.Company,
				"match_score": l.MatchScore, "salary_string": l.SalaryString,
			})
		}
	}
	if highMatchLeads == nil {
		highMatchLeads = []map[string]interface{}{}
	}
	return map[string]interface{}{
		"last_run":         lastRun,
		"new_leads":        result.NewLeads,
		"duplicates":       result.Duplicates,
		"high_matches":     result.HighMatches,
		"high_match_leads": highMatchLeads,
	}
}

// --- Legacy platform sweep stubs (used by server.go endpoints) ---

// LegacySweepResult holds the outcome of a single-platform sweep.
type LegacySweepResult struct {
	Platform   string   `json:"platform"`
	NewLeads   int      `json:"newLeads"`
	Duplicates int      `json:"duplicates"`
	Errors     []string `json:"errors"`
}

// Sweep runs a lead discovery sweep across the given platforms.
// This is a stub — actual crawling is done via RunSweep with TheirStack.
func Sweep(platforms []string, st *store.Store) ([]LegacySweepResult, error) {
	var results []LegacySweepResult
	for _, p := range platforms {
		results = append(results, LegacySweepResult{
			Platform:   p,
			NewLeads:   0,
			Duplicates: 0,
			Errors:     []string{"sweep not yet implemented — use RunSweep with TheirStack"},
		})
	}
	return results, nil
}

// SweepStatus returns the current sweep status for each known platform.
func SweepStatus() map[string]string {
	return map[string]string{
		"linkedin":  "idle",
		"indeed":    "idle",
		"upwork":    "idle",
		"toptal":    "idle",
		"wellfound": "idle",
	}
}

// --- CDPClient (moved from cdp.go) ---

// CDPClient connects to Chrome DevTools Protocol for browser automation.
type CDPClient struct {
	endpoint string // ws://127.0.0.1:9222
}

// NewCDPClient creates a new CDP client pointing at the given endpoint.
func NewCDPClient(endpoint string) *CDPClient {
	return &CDPClient{endpoint: endpoint}
}

// Available checks if the CDP endpoint is reachable by querying /json/version.
// Returns false gracefully if the endpoint is not available.
func (c *CDPClient) Available() bool {
	if c.endpoint == "" {
		return false
	}
	// Convert ws:// to http:// for the version check.
	httpURL := c.endpoint
	if len(httpURL) > 5 && httpURL[:5] == "ws://" {
		httpURL = "http://" + httpURL[5:]
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(httpURL + "/json/version")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// Endpoint returns the configured CDP endpoint URL.
func (c *CDPClient) Endpoint() string {
	return c.endpoint
}
