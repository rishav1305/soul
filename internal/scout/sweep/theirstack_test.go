package sweep

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// mockTransport is an http.RoundTripper that returns a preset response.
type mockTransport struct {
	response string
	status   int
	lastBody string
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(req.Body)
	m.lastBody = string(body)
	return &http.Response{
		StatusCode: m.status,
		Body:       io.NopCloser(strings.NewReader(m.response)),
		Header:     make(http.Header),
	}, nil
}

func newMockClient(status int, response string) (*TheirStackClient, *mockTransport) {
	mt := &mockTransport{status: status, response: response}
	hc := &http.Client{Transport: mt}
	return NewTheirStackClient("test-api-key", hc), mt
}

const sampleJobResponse = `{
	"data": [
		{
			"id": 42,
			"job_title": "Senior Go Engineer",
			"url": "https://example.com/jobs/42",
			"final_url": "https://example.com/jobs/42/apply",
			"source_url": "https://linkedin.com/jobs/42",
			"date_posted": "2026-03-15",
			"discovered_at": "2026-03-16T10:00:00Z",
			"company": "Acme Corp",
			"location": "Remote, US",
			"short_location": "US",
			"country": "United States",
			"country_code": "US",
			"remote": true,
			"hybrid": false,
			"salary_string": "$150k - $180k",
			"min_annual_salary_usd": 150000,
			"max_annual_salary_usd": 180000,
			"salary_currency": "USD",
			"seniority": "senior",
			"employment_statuses": ["full_time"],
			"easy_apply": false,
			"technology_slugs": ["go", "postgresql"],
			"keyword_slugs": ["backend", "distributed"],
			"description": "We are looking for a senior Go engineer...",
			"normalized_title": "software engineer",
			"company_domain": "acme.com",
			"company_object": {
				"domain": "acme.com",
				"industry": "Software",
				"employee_count": 500,
				"linkedin_url": "https://linkedin.com/company/acme",
				"total_funding_usd": 50000000,
				"funding_stage": "Series B",
				"logo": "https://acme.com/logo.png",
				"country": "US"
			},
			"hiring_team": [
				{
					"full_name": "Jane Doe",
					"linkedin_url": "https://linkedin.com/in/janedoe",
					"role": "Engineering Manager"
				}
			]
		}
	],
	"metadata": {
		"total_results": 1
	}
}`

func TestTheirStackClient_Search(t *testing.T) {
	client, mt := newMockClient(http.StatusOK, sampleJobResponse)

	remote := true
	cfg := &SweepConfig{
		JobTitleOr:          []string{"software engineer", "golang developer"},
		JobCountryCodeOr:    []string{"US", "IN"},
		JobTechnologySlugOr: []string{"go", "react"},
		Remote:              &remote,
		PostedAtMaxAgeDays:  7,
		Limit:               50,
	}

	resp, err := client.Search(cfg, "2026-03-10T00:00:00Z", 0)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Metadata.TotalResults != 1 {
		t.Errorf("expected TotalResults=1, got %d", resp.Metadata.TotalResults)
	}
	if len(resp.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(resp.Jobs))
	}

	job := resp.Jobs[0]
	if job.ID != 42 {
		t.Errorf("expected ID=42, got %d", job.ID)
	}
	if job.JobTitle != "Senior Go Engineer" {
		t.Errorf("unexpected job title: %q", job.JobTitle)
	}
	if !job.Remote {
		t.Error("expected Remote=true")
	}
	if job.MinAnnualSalaryUSD != 150000 {
		t.Errorf("expected MinAnnualSalaryUSD=150000, got %f", job.MinAnnualSalaryUSD)
	}
	if job.CompanyObject == nil {
		t.Fatal("expected non-nil CompanyObject")
	}
	if job.CompanyObject.FundingStage != "Series B" {
		t.Errorf("unexpected FundingStage: %q", job.CompanyObject.FundingStage)
	}
	if len(job.HiringTeam) != 1 {
		t.Fatalf("expected 1 hiring team member, got %d", len(job.HiringTeam))
	}
	if job.HiringTeam[0].FullName != "Jane Doe" {
		t.Errorf("unexpected HiringTeam[0].FullName: %q", job.HiringTeam[0].FullName)
	}

	// Verify cursor was sent in the request body.
	if !strings.Contains(mt.lastBody, "discovered_at_gte") {
		t.Errorf("expected discovered_at_gte in request body, got: %s", mt.lastBody)
	}
	if !strings.Contains(mt.lastBody, "2026-03-10T00:00:00Z") {
		t.Errorf("expected cursor value in request body, got: %s", mt.lastBody)
	}
}

func TestTheirStackClient_Search_NoCursor(t *testing.T) {
	client, mt := newMockClient(http.StatusOK, `{"data":[],"metadata":{"total_results":0}}`)

	resp, err := client.Search(DefaultConfig(), "", 0)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(resp.Jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(resp.Jobs))
	}

	// When cursor is empty, discovered_at_gte should NOT appear in request body.
	if strings.Contains(mt.lastBody, "discovered_at_gte") {
		t.Errorf("expected discovered_at_gte to be omitted, got body: %s", mt.lastBody)
	}
}

func TestTheirStackClient_Search_RateLimit(t *testing.T) {
	client, _ := newMockClient(http.StatusTooManyRequests, `{"error":"rate limit exceeded"}`)

	_, err := client.Search(DefaultConfig(), "", 0)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !IsRateLimitError(err) {
		t.Errorf("expected RateLimitError, got: %T: %v", err, err)
	}
}

func TestTheirStackClient_Search_CreditsExhausted(t *testing.T) {
	client, _ := newMockClient(http.StatusPaymentRequired, `{"error":"credits exhausted"}`)

	_, err := client.Search(DefaultConfig(), "", 0)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !IsCreditsExhaustedError(err) {
		t.Errorf("expected CreditsExhaustedError, got: %T: %v", err, err)
	}
}

func TestTheirStackClient_Search_ServerError(t *testing.T) {
	client, _ := newMockClient(http.StatusInternalServerError, `{"error":"internal server error"}`)

	_, err := client.Search(DefaultConfig(), "", 0)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if IsRateLimitError(err) || IsCreditsExhaustedError(err) {
		t.Errorf("expected generic error, got typed error: %T", err)
	}
}

func TestTheirStackClient_AuthHeader(t *testing.T) {
	mt := &mockTransport{status: http.StatusOK, response: `{"data":[],"metadata":{"total_results":0}}`}

	// Use a custom transport that captures the request.
	captureTransport := &captureHeaderTransport{inner: mt}
	hc := &http.Client{Transport: captureTransport}
	client := NewTheirStackClient("my-secret-key", hc)

	_, err := client.Search(nil, "", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captureTransport.lastReq == nil {
		t.Fatal("expected request to be captured")
	}
	auth := captureTransport.lastReq.Header.Get("Authorization")
	if auth != "Bearer my-secret-key" {
		t.Errorf("expected Authorization header 'Bearer my-secret-key', got %q", auth)
	}
}

type captureHeaderTransport struct {
	inner   *mockTransport
	lastReq *http.Request
}

func (c *captureHeaderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	c.lastReq = req
	return c.inner.RoundTrip(req)
}

func TestInferPipeline(t *testing.T) {
	tests := []struct {
		name     string
		statuses []string
		want     string
	}{
		{
			name:     "full_time maps to job",
			statuses: []string{"full_time"},
			want:     "job",
		},
		{
			name:     "contract maps to contract",
			statuses: []string{"contract"},
			want:     "contract",
		},
		{
			name:     "part_time maps to freelance",
			statuses: []string{"part_time"},
			want:     "freelance",
		},
		{
			name:     "temporary maps to freelance",
			statuses: []string{"temporary"},
			want:     "freelance",
		},
		{
			name:     "internship maps to job",
			statuses: []string{"internship"},
			want:     "job",
		},
		{
			name:     "mixed full_time and contract: contract wins",
			statuses: []string{"full_time", "contract"},
			want:     "contract",
		},
		{
			name:     "empty slice maps to job",
			statuses: []string{},
			want:     "job",
		},
		{
			name:     "unknown status maps to job",
			statuses: []string{"unknown_status"},
			want:     "job",
		},
		{
			name:     "nil slice maps to job",
			statuses: nil,
			want:     "job",
		},
		{
			name:     "contract beats part_time",
			statuses: []string{"part_time", "contract"},
			want:     "contract",
		},
		{
			name:     "full_time beats part_time",
			statuses: []string{"part_time", "full_time"},
			want:     "job",
		},
		{
			name:     "internship with unknown",
			statuses: []string{"unknown", "internship"},
			want:     "job",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := InferPipeline(tc.statuses)
			if got != tc.want {
				t.Errorf("InferPipeline(%v) = %q, want %q", tc.statuses, got, tc.want)
			}
		})
	}
}
