package sweep

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// TheirStackClient is an HTTP client for the TheirStack jobs API.
type TheirStackClient struct {
	apiKey     string
	httpClient *http.Client
}

// NewTheirStackClient creates a new TheirStackClient with the given API key and HTTP client.
// If httpClient is nil, http.DefaultClient is used.
func NewTheirStackClient(apiKey string, httpClient *http.Client) *TheirStackClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &TheirStackClient{
		apiKey:     apiKey,
		httpClient: httpClient,
	}
}

// SearchResponse is the top-level response from the TheirStack jobs search API.
type SearchResponse struct {
	Jobs     []Job        `json:"data"`
	Metadata ResponseMeta `json:"metadata"`
}

// ResponseMeta holds pagination/total metadata from the API.
type ResponseMeta struct {
	TotalResults int `json:"total_results"`
}

// Job represents a single job listing returned by TheirStack.
type Job struct {
	ID                 int            `json:"id"`
	JobTitle           string         `json:"job_title"`
	URL                string         `json:"url"`
	FinalURL           string         `json:"final_url"`
	SourceURL          string         `json:"source_url"`
	DatePosted         string         `json:"date_posted"`
	DiscoveredAt       string         `json:"discovered_at"`
	Company            string         `json:"company"`
	Location           string         `json:"location"`
	ShortLocation      string         `json:"short_location"`
	Country            string         `json:"country"`
	CountryCode        string         `json:"country_code"`
	Remote             bool           `json:"remote"`
	Hybrid             bool           `json:"hybrid"`
	SalaryString       string         `json:"salary_string"`
	MinAnnualSalaryUSD float64        `json:"min_annual_salary_usd"`
	MaxAnnualSalaryUSD float64        `json:"max_annual_salary_usd"`
	SalaryCurrency     string         `json:"salary_currency"`
	Seniority          string         `json:"seniority"`
	EmploymentStatuses []string       `json:"employment_statuses"`
	EasyApply          bool           `json:"easy_apply"`
	TechnologySlugs    []string       `json:"technology_slugs"`
	KeywordSlugs       []string       `json:"keyword_slugs"`
	Description        string         `json:"description"`
	NormalizedTitle    string         `json:"normalized_title"`
	CompanyDomain      string         `json:"company_domain"`
	CompanyObject      *CompanyInfo   `json:"company_object"`
	HiringTeam         []HiringPerson `json:"hiring_team"`
}

// CompanyInfo holds enriched company data attached to a job listing.
type CompanyInfo struct {
	Domain          string  `json:"domain"`
	Industry        string  `json:"industry"`
	EmployeeCount   int     `json:"employee_count"`
	LinkedInURL     string  `json:"linkedin_url"`
	TotalFundingUSD float64 `json:"total_funding_usd"`
	FundingStage    string  `json:"funding_stage"`
	Logo            string  `json:"logo"`
	Country         string  `json:"country"`
}

// HiringPerson represents a member of the hiring team listed on a job posting.
type HiringPerson struct {
	FullName    string `json:"full_name"`
	LinkedInURL string `json:"linkedin_url"`
	Role        string `json:"role"`
}

// RateLimitError is returned when the API responds with HTTP 429.
type RateLimitError struct{ Message string }

func (e *RateLimitError) Error() string { return e.Message }

// CreditsExhaustedError is returned when the API responds with HTTP 402.
type CreditsExhaustedError struct{ Message string }

func (e *CreditsExhaustedError) Error() string { return e.Message }

// IsRateLimitError reports whether err is a *RateLimitError.
func IsRateLimitError(err error) bool {
	_, ok := err.(*RateLimitError)
	return ok
}

// IsCreditsExhaustedError reports whether err is a *CreditsExhaustedError.
func IsCreditsExhaustedError(err error) bool {
	_, ok := err.(*CreditsExhaustedError)
	return ok
}

// searchRequest is the JSON body sent to the TheirStack search endpoint.
type searchRequest struct {
	JobTitleOr           []string `json:"job_title_or,omitempty"`
	JobTitleNot          []string `json:"job_title_not,omitempty"`
	JobCountryCodeOr     []string `json:"job_country_code_or,omitempty"`
	JobTechnologySlugOr  []string `json:"job_technology_slug_or,omitempty"`
	JobLocationPatternOr []string `json:"job_location_pattern_or,omitempty"`
	Remote               *bool    `json:"remote,omitempty"`
	SeniorityOr          []string `json:"seniority_or,omitempty"`
	MinSalaryUSD         *float64 `json:"min_salary_usd,omitempty"`
	PostedAtMaxAgeDays   int      `json:"posted_at_max_age_days,omitempty"`
	Limit                int      `json:"limit,omitempty"`
	Offset               int      `json:"offset"`
	DiscoveredAtGte      string   `json:"discovered_at_gte,omitempty"`
}

const theirStackEndpoint = "https://api.theirstack.com/v1/jobs/search"

// Search queries the TheirStack API using the given SweepConfig, cursor, and offset.
// cursor is an ISO-8601 datetime string used as discovered_at_gte; pass "" to omit.
// Returns RateLimitError on HTTP 429, CreditsExhaustedError on HTTP 402,
// and a generic error for other non-2xx responses.
func (c *TheirStackClient) Search(cfg *SweepConfig, cursor string, offset int) (*SearchResponse, error) {
	body := searchRequest{
		Offset: offset,
	}
	if cfg != nil {
		body.JobTitleOr = cfg.JobTitleOr
		body.JobTitleNot = cfg.JobTitleNot
		body.JobCountryCodeOr = cfg.JobCountryCodeOr
		body.JobTechnologySlugOr = cfg.JobTechnologySlugOr
		body.JobLocationPatternOr = cfg.JobLocationPatternOr
		body.Remote = cfg.Remote
		body.SeniorityOr = cfg.SeniorityOr
		body.MinSalaryUSD = cfg.MinSalaryUSD
		if cfg.PostedAtMaxAgeDays > 0 {
			body.PostedAtMaxAgeDays = cfg.PostedAtMaxAgeDays
		}
		if cfg.Limit > 0 {
			body.Limit = cfg.Limit
		}
	}
	if cursor != "" {
		body.DiscoveredAtGte = cursor
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("theirstack: marshal request: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, theirStackEndpoint, bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("theirstack: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("theirstack: do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("theirstack: read response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
		// success — fall through to unmarshal
	case http.StatusTooManyRequests:
		return nil, &RateLimitError{Message: fmt.Sprintf("theirstack: rate limit exceeded (429): %s", string(respBody))}
	case http.StatusPaymentRequired:
		return nil, &CreditsExhaustedError{Message: fmt.Sprintf("theirstack: credits exhausted (402): %s", string(respBody))}
	default:
		return nil, fmt.Errorf("theirstack: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var result SearchResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("theirstack: unmarshal response: %w", err)
	}
	return &result, nil
}

// InferPipeline determines the pipeline type from a job's employment statuses.
// Rules (first match wins):
//  1. contains "contract"  → "contract"
//  2. contains "full_time" → "job"
//  3. contains "part_time" or "temporary" → "freelance"
//  4. contains "internship" → "job"
//  5. empty / unknown → "job"
func InferPipeline(statuses []string) string {
	for _, s := range statuses {
		if s == "contract" {
			return "contract"
		}
	}
	for _, s := range statuses {
		if s == "full_time" {
			return "job"
		}
	}
	for _, s := range statuses {
		if s == "part_time" || s == "temporary" {
			return "freelance"
		}
	}
	for _, s := range statuses {
		if s == "internship" {
			return "job"
		}
	}
	return "job"
}
