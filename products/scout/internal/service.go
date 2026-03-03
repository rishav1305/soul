package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rishav1305/soul/products/scout/internal/browser"
	"github.com/rishav1305/soul/products/scout/internal/data"
	"github.com/rishav1305/soul/products/scout/internal/generate"
	"github.com/rishav1305/soul/products/scout/internal/profiledb"
	"github.com/rishav1305/soul/products/scout/internal/supabase"
	syncpkg "github.com/rishav1305/soul/products/scout/internal/sync"
	sweeppkg "github.com/rishav1305/soul/products/scout/internal/sweep"
	soulv1 "github.com/rishav1305/soul/proto/soul/v1"
	"google.golang.org/grpc"
)

// ScoutService implements the soulv1.ProductServiceServer interface,
// exposing the scout job-hunting automation tools over gRPC.
type ScoutService struct {
	soulv1.UnimplementedProductServiceServer
	store *data.Store
}

// scoutConfig mirrors the JSON structure of ~/.soul/scout/config.json.
type scoutConfig struct {
	RemoteBrowser *struct {
		Enabled     bool   `json:"enabled"`
		ManagerURL  string `json:"manager_url"`
		ProfileBase string `json:"profile_base"`
	} `json:"remote_browser,omitempty"`
}

// NewScoutService creates a ScoutService with an initialised data store.
func NewScoutService() *ScoutService {
	store, err := data.NewStore()
	if err != nil {
		log.Printf("[scout] WARNING: data store init failed: %v", err)
	}

	// Load remote browser config if present.
	if store != nil {
		cfgPath := filepath.Join(store.DataDir(), "config.json")
		if raw, err := os.ReadFile(cfgPath); err == nil {
			var cfg scoutConfig
			if err := json.Unmarshal(raw, &cfg); err == nil && cfg.RemoteBrowser != nil && cfg.RemoteBrowser.Enabled {
				browser.SetRemoteConfig(browser.RemoteConfig{
					Enabled:     true,
					ManagerURL:  cfg.RemoteBrowser.ManagerURL,
					ProfileBase: cfg.RemoteBrowser.ProfileBase,
				})
				log.Printf("[scout] remote browser enabled: %s", cfg.RemoteBrowser.ManagerURL)
			}
		}
	}

	return &ScoutService{store: store}
}

// GetManifest returns the product manifest describing the scout product
// and its available tools.
func (s *ScoutService) GetManifest(_ context.Context, _ *soulv1.Empty) (*soulv1.Manifest, error) {
	return &soulv1.Manifest{
		Name:    "scout",
		Version: "0.1.0",
		Tools: []*soulv1.Tool{
			{
				Name:        "setup",
				Description: "Open visible browser for platform login",
				InputSchemaJson: `{
					"type": "object",
					"properties": {
						"platform": {"type": "string", "description": "Platform to log into (e.g. linkedin, indeed, glassdoor)"},
						"headless": {"type": "boolean", "description": "Run browser in headless mode (default false)"}
					},
					"required": ["platform"]
				}`,
			},
			{
				Name:        "sync",
				Description: "Compare Supabase profile vs live platforms",
				InputSchemaJson: `{
					"type": "object",
					"properties": {
						"platforms": {"type": "array", "items": {"type": "string"}, "description": "Platforms to sync (e.g. linkedin, indeed)"},
						"profileId": {"type": "string", "description": "Supabase profile ID to compare against"}
					},
					"required": ["platforms"]
				}`,
			},
			{
				Name:        "sweep",
				Description: "Check job portals for opportunities",
				InputSchemaJson: `{
					"type": "object",
					"properties": {
						"platforms": {"type": "array", "items": {"type": "string"}, "description": "Platforms to sweep"},
						"keywords": {"type": "array", "items": {"type": "string"}, "description": "Job search keywords"},
						"location": {"type": "string", "description": "Job location filter"},
						"limit": {"type": "integer", "description": "Maximum number of results per platform"}
					},
					"required": ["platforms", "keywords"]
				}`,
			},
			{
				Name:        "generate",
				Description: "Create tailored resume and cover letter PDF",
				InputSchemaJson: `{
					"type": "object",
					"properties": {
						"jobUrl": {"type": "string", "description": "URL of the job posting to tailor for"},
						"jobDescription": {"type": "string", "description": "Raw job description text (alternative to jobUrl)"},
						"profileId": {"type": "string", "description": "Supabase profile ID for resume data"},
						"outputDir": {"type": "string", "description": "Directory to write generated PDFs"}
					},
					"required": []
				}`,
			},
			{
				Name:        "track",
				Description: "Application CRUD — create, read, update, delete tracked applications",
				InputSchemaJson: `{
					"type": "object",
					"properties": {
						"action": {"type": "string", "enum": ["create", "read", "update", "delete", "list"], "description": "CRUD action to perform"},
						"applicationId": {"type": "string", "description": "Application ID (for read/update/delete)"},
						"data": {"type": "object", "description": "Application data (for create/update)", "properties": {
							"company": {"type": "string"},
							"role": {"type": "string"},
							"url": {"type": "string"},
							"status": {"type": "string", "enum": ["saved", "applied", "interviewing", "offered", "rejected", "withdrawn"]},
							"notes": {"type": "string"}
						}}
					},
					"required": ["action"]
				}`,
			},
			{
				Name:        "report",
				Description: "Generate structured dashboard JSON with application stats",
				InputSchemaJson: `{
					"type": "object",
					"properties": {
						"format": {"type": "string", "enum": ["summary", "detailed", "weekly"], "description": "Report format"},
						"dateRange": {"type": "object", "properties": {
							"from": {"type": "string", "description": "Start date (ISO 8601)"},
							"to": {"type": "string", "description": "End date (ISO 8601)"}
						}, "description": "Optional date range filter"}
					},
					"required": []
				}`,
			},
			{
				Name:        "profile",
				Description: "Return the full professional profile from the local profile database as structured JSON",
				InputSchemaJson: `{
					"type": "object",
					"properties": {
						"sections": {"type": "array", "items": {"type": "string"}, "description": "Return only these sections (e.g. ['experience','skills']). Empty = all 13 tables."}
					}
				}`,
			},
			{
				Name:        "profile_pull",
				Description: "Pull profile data from Supabase cloud into the local profile database",
				InputSchemaJson: `{
					"type": "object",
					"properties": {
						"tables": {"type": "array", "items": {"type": "string"}, "description": "Tables to pull. Empty = all 13 tables."}
					}
				}`,
			},
			{
				Name:        "profile_push",
				Description: "Push local profile database to Supabase cloud (overwrites live website data)",
				InputSchemaJson: `{
					"type": "object",
					"properties": {
						"tables": {"type": "array", "items": {"type": "string"}, "description": "Tables to push. Empty = all 13 tables."},
						"confirm": {"type": "boolean", "description": "Must be true to actually push. Safety gate to prevent accidental overwrites."}
					},
					"required": ["confirm"]
				}`,
			},
		},
	}, nil
}

// setupInput holds the parsed JSON input for the setup tool.
type setupInput struct {
	Platform string `json:"platform"`
	Headless bool   `json:"headless"`
}

// syncInput holds the parsed JSON input for the sync tool.
type syncInput struct {
	Platforms []string `json:"platforms"`
	ProfileID string   `json:"profileId,omitempty"`
}

// sweepInput holds the parsed JSON input for the sweep tool.
type sweepInput struct {
	Platforms []string `json:"platforms"`
	Keywords  []string `json:"keywords"`
	Location  string   `json:"location,omitempty"`
	Limit     int      `json:"limit,omitempty"`
}

// generateInput holds the parsed JSON input for the generate tool.
type generateInput struct {
	Variant       string `json:"variant"`
	Company       string `json:"company,omitempty"`
	Role          string `json:"role,omitempty"`
	JobURL        string `json:"jobUrl,omitempty"`
	SpecificThing string `json:"specific_thing,omitempty"`
}

// trackInput holds the parsed JSON input for the track tool.
type trackInput struct {
	Action       string `json:"action"`
	ID           string `json:"id,omitempty"`
	Company      string `json:"company,omitempty"`
	Role         string `json:"role,omitempty"`
	Platform     string `json:"platform,omitempty"`
	Variant      string `json:"variant,omitempty"`
	Status       string `json:"status,omitempty"`
	FollowUp     string `json:"follow_up,omitempty"`
	Notes        string `json:"notes,omitempty"`
	FilterStatus string `json:"filter_status,omitempty"`
}

// reportInput holds the parsed JSON input for the report tool.
type reportInput struct {
	Format    string          `json:"format,omitempty"`
	DateRange json.RawMessage `json:"dateRange,omitempty"`
}

// profileInput holds the parsed JSON input for the profile tool.
type profileInput struct {
	Sections []string `json:"sections"`
}

// profilePullInput holds the parsed JSON input for the profile_pull tool.
type profilePullInput struct {
	Tables []string `json:"tables"`
}

// profilePushInput holds the parsed JSON input for the profile_push tool.
type profilePushInput struct {
	Tables  []string `json:"tables"`
	Confirm bool     `json:"confirm"`
}

// ExecuteTool routes incoming tool requests to the appropriate handler.
func (s *ScoutService) ExecuteTool(_ context.Context, req *soulv1.ToolRequest) (*soulv1.ToolResponse, error) {
	switch req.Tool {
	case "setup":
		return s.executeSetup(req.InputJson)
	case "sync":
		return s.executeSync(req.InputJson)
	case "sweep":
		return s.executeSweep(req.InputJson)
	case "generate":
		return s.executeGenerate(req.InputJson)
	case "track":
		return s.executeTrack(req.InputJson)
	case "report":
		return s.executeReport(req.InputJson)
	case "profile":
		return s.executeProfile(req.InputJson)
	case "profile_pull":
		return s.executeProfilePull(req.InputJson)
	case "profile_push":
		return s.executeProfilePush(req.InputJson)
	default:
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("unknown tool: %s", req.Tool),
		}, nil
	}
}

func (s *ScoutService) executeSetup(inputJSON string) (*soulv1.ToolResponse, error) {
	var input setupInput
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("invalid input: %v", err),
		}, nil
	}

	if input.Platform == "" {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  "platform is required",
		}, nil
	}

	if _, ok := browser.PlatformURLs[input.Platform]; !ok {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("unknown platform: %s", input.Platform),
		}, nil
	}

	cmd, err := browser.LaunchNative(input.Platform)
	if err != nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("failed to launch browser: %v", err),
		}, nil
	}

	// Wait for the user to close Chrome after logging in.
	if err := cmd.Wait(); err != nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("browser exited with error: %v", err),
		}, nil
	}

	profileDir, _ := browser.ProfileDir(input.Platform)
	if browser.HasProfile(input.Platform) {
		return &soulv1.ToolResponse{
			Success: true,
			Output:  fmt.Sprintf("Login session saved for %s at %s", input.Platform, profileDir),
		}, nil
	}

	return &soulv1.ToolResponse{
		Success: false,
		Output:  fmt.Sprintf("No session data found for %s after browser closed", input.Platform),
	}, nil
}

func (s *ScoutService) executeSync(inputJSON string) (*soulv1.ToolResponse, error) {
	var input syncInput
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("invalid input: %v", err),
		}, nil
	}

	// Expand "all" to all platforms plus website and github.
	platforms := input.Platforms
	for _, p := range platforms {
		if strings.ToLower(p) == "all" {
			platforms = append(browser.AllPlatforms(), "website", "github")
			break
		}
	}

	// Fetch profile data from Supabase.
	client, err := supabase.NewClient()
	if err != nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("supabase client error: %v", err),
		}, nil
	}

	profile, err := client.GetProfileData()
	if err != nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("fetch profile error: %v", err),
		}, nil
	}

	var results []data.PlatformSync
	driftCount := 0

	for _, platform := range platforms {
		result := syncpkg.CheckPlatform(platform, profile)
		results = append(results, result.Platform)
		if result.Platform.Status == "drift" {
			driftCount++
		}
	}

	if s.store != nil {
		if err := s.store.SetSyncResults(results); err != nil {
			log.Printf("[scout] warning: failed to save sync results: %v", err)
		}
	}

	syncedCount := len(results) - driftCount
	return &soulv1.ToolResponse{
		Success: true,
		Output:  fmt.Sprintf("Sync complete: %d/%d platforms in sync, %d with drift", syncedCount, len(results), driftCount),
	}, nil
}

func (s *ScoutService) executeSweep(inputJSON string) (*soulv1.ToolResponse, error) {
	var input sweepInput
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("invalid input: %v", err),
		}, nil
	}

	// Expand "all" to all platforms.
	platforms := input.Platforms
	for _, p := range platforms {
		if strings.ToLower(p) == "all" {
			platforms = browser.AllPlatforms()
			break
		}
	}

	// Fetch profile data for keyword/location defaults.
	client, err := supabase.NewClient()
	if err != nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("supabase client error: %v", err),
		}, nil
	}

	profile, err := client.GetProfileData()
	if err != nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("fetch profile error: %v", err),
		}, nil
	}

	var allOpps []data.Opportunity
	var allMsgs []data.Message

	for _, platform := range platforms {
		result := sweeppkg.SweepPlatform(platform, profile, input.Keywords, input.Location)
		if result.Error != nil {
			log.Printf("[scout] sweep %s error: %v", platform, result.Error)
			continue
		}
		allOpps = append(allOpps, result.Opportunities...)
		allMsgs = append(allMsgs, result.Messages...)
	}

	if s.store != nil {
		if err := s.store.SetSweepResults(allOpps, allMsgs); err != nil {
			log.Printf("[scout] warning: failed to save sweep results: %v", err)
		}
	}

	return &soulv1.ToolResponse{
		Success: true,
		Output:  fmt.Sprintf("Sweep complete: found %d opportunities across %d platforms", len(allOpps), len(platforms)),
	}, nil
}

func (s *ScoutService) executeGenerate(inputJSON string) (*soulv1.ToolResponse, error) {
	var input generateInput
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("invalid input: %v", err),
		}, nil
	}

	// Look up the variant.
	variant, ok := generate.Variants[strings.ToUpper(input.Variant)]
	if !ok {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("unknown variant %q — valid variants: A, B, C, D, E, F, G", input.Variant),
		}, nil
	}

	// Fetch profile data from Supabase.
	client, err := supabase.NewClient()
	if err != nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("supabase client error: %v", err),
		}, nil
	}

	profile, err := client.GetProfileData()
	if err != nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("fetch profile error: %v", err),
		}, nil
	}

	// Build output paths.
	company := input.Company
	if company == "" {
		company = "general"
	}
	role := input.Role
	if role == "" {
		role = variant.TargetRole
	}

	slug := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(company, " ", "-"), "/", "-"))
	dateStr := time.Now().Format("2006-01-02")
	variantID := strings.ToLower(variant.ID)

	draftsDir := filepath.Join(s.store.DataDir(), "drafts")
	resumePath := filepath.Join(draftsDir, fmt.Sprintf("%s-%s-%s-resume.pdf", slug, variantID, dateStr))
	coverPath := filepath.Join(draftsDir, fmt.Sprintf("%s-%s-%s-cover.pdf", slug, variantID, dateStr))

	var artifacts []*soulv1.Artifact

	// Build and generate resume PDF.
	resumeHTML, err := generate.BuildResumeHTML(variant, profile)
	if err != nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("build resume HTML error: %v", err),
		}, nil
	}

	if err := generate.HTMLToPDF(resumeHTML, resumePath); err != nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("generate resume PDF error: %v", err),
		}, nil
	}
	artifacts = append(artifacts, &soulv1.Artifact{
		Type: "pdf",
		Path: resumePath,
	})

	// Build and generate cover letter PDF.
	specificThing := input.SpecificThing
	if specificThing == "" {
		specificThing = "Your focus on AI innovation"
	}

	coverHTML, err := generate.BuildCoverHTML(variant, profile, company, role, specificThing)
	if err != nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("build cover HTML error: %v", err),
		}, nil
	}

	if err := generate.HTMLToPDF(coverHTML, coverPath); err != nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("generate cover PDF error: %v", err),
		}, nil
	}
	artifacts = append(artifacts, &soulv1.Artifact{
		Type: "pdf",
		Path: coverPath,
	})

	return &soulv1.ToolResponse{
		Success:   true,
		Output:    fmt.Sprintf("Generated variant %s (%s) resume and cover letter for %s. Files: %s, %s", variant.ID, variant.TargetRole, company, resumePath, coverPath),
		Artifacts: artifacts,
	}, nil
}

func (s *ScoutService) executeTrack(inputJSON string) (*soulv1.ToolResponse, error) {
	var input trackInput
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("invalid input: %v", err),
		}, nil
	}

	if s.store == nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  "data store not initialised",
		}, nil
	}

	switch input.Action {
	case "add":
		app := data.Application{
			Company:  input.Company,
			Role:     input.Role,
			Platform: input.Platform,
			Variant:  input.Variant,
			Status:   input.Status,
			FollowUp: input.FollowUp,
			Notes:    input.Notes,
		}
		if app.Status == "" {
			app.Status = "applied"
		}
		if err := s.store.AddApplication(app); err != nil {
			return &soulv1.ToolResponse{
				Success: false,
				Output:  fmt.Sprintf("failed to add application: %v", err),
			}, nil
		}
		return &soulv1.ToolResponse{
			Success: true,
			Output:  fmt.Sprintf("Added application: %s at %s", input.Role, input.Company),
		}, nil

	case "update":
		if input.ID == "" {
			return &soulv1.ToolResponse{
				Success: false,
				Output:  "id is required for update action",
			}, nil
		}
		if err := s.store.UpdateApplication(input.ID, input.Status, input.FollowUp, input.Notes); err != nil {
			return &soulv1.ToolResponse{
				Success: false,
				Output:  fmt.Sprintf("failed to update application: %v", err),
			}, nil
		}
		return &soulv1.ToolResponse{
			Success: true,
			Output:  fmt.Sprintf("Updated application %s", input.ID),
		}, nil

	case "list":
		apps := s.store.ListApplications(input.FilterStatus)
		appsJSON, err := json.Marshal(apps)
		if err != nil {
			return &soulv1.ToolResponse{
				Success: false,
				Output:  fmt.Sprintf("failed to marshal applications: %v", err),
			}, nil
		}
		summary := fmt.Sprintf("Found %d application(s)", len(apps))
		if input.FilterStatus != "" {
			summary += fmt.Sprintf(" with status %q", input.FilterStatus)
		}
		return &soulv1.ToolResponse{
			Success:        true,
			Output:         summary,
			StructuredJson: string(appsJSON),
		}, nil

	default:
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("unknown track action: %s (expected add, update, or list)", input.Action),
		}, nil
	}
}

func (s *ScoutService) executeReport(inputJSON string) (*soulv1.ToolResponse, error) {
	var input reportInput
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("invalid input: %v", err),
		}, nil
	}

	if s.store == nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  "data store not initialised",
		}, nil
	}

	d := s.store.GetReportData()

	// --- sync section ---
	inSync := 0
	drift := 0
	var syncDetails []map[string]interface{}
	for _, r := range d.Sync.Results {
		detail := map[string]interface{}{
			"platform":  r.Platform,
			"status":    r.Status,
			"issues":    r.Issues,
			"checkedAt": r.CheckedAt,
		}
		if len(r.Details) > 0 {
			detail["details"] = r.Details
		}
		syncDetails = append(syncDetails, detail)
		if r.Status == "synced" {
			inSync++
		} else {
			drift++
		}
	}
	syncSection := map[string]interface{}{
		"last_run":           d.Sync.LastRun,
		"platforms_checked":  len(d.Sync.Results),
		"in_sync":           inSync,
		"drift":             drift,
		"details":           syncDetails,
	}

	// --- sweep section ---
	var opps []map[string]interface{}
	for _, o := range d.Sweep.Opportunities {
		opp := map[string]interface{}{
			"id":       o.ID,
			"company":  o.Company,
			"role":     o.Role,
			"platform": o.Platform,
			"match":    o.Match,
			"url":      o.URL,
			"foundAt":  o.FoundAt,
		}
		if o.Location != "" {
			opp["location"] = o.Location
		}
		if o.PostedAt != "" {
			opp["postedAt"] = o.PostedAt
		}
		opps = append(opps, opp)
	}
	sweepSection := map[string]interface{}{
		"last_run":          d.Sweep.LastRun,
		"new_opportunities": len(d.Sweep.Opportunities),
		"messages":          len(d.Sweep.Messages),
		"opportunities":     opps,
	}

	// --- applications section ---
	byStatus := make(map[string]int)
	active := 0
	for _, a := range d.Applications {
		byStatus[a.Status]++
		switch a.Status {
		case "rejected", "withdrawn", "offer":
			// not active
		default:
			active++
		}
	}
	// recent: last 10 applications
	recent := d.Applications
	if len(recent) > 10 {
		recent = recent[len(recent)-10:]
	}
	var recentList []map[string]interface{}
	for _, a := range recent {
		recentList = append(recentList, map[string]interface{}{
			"id":        a.ID,
			"company":   a.Company,
			"role":      a.Role,
			"status":    a.Status,
			"appliedAt": a.AppliedAt,
			"updatedAt": a.UpdatedAt,
		})
	}
	applicationsSection := map[string]interface{}{
		"total":     len(d.Applications),
		"active":    active,
		"by_status": byStatus,
		"recent":    recentList,
	}

	// --- metrics section ---
	metricsSection := d.Metrics

	// --- follow_ups section ---
	today := time.Now().Format("2006-01-02")
	var followUps []map[string]interface{}
	for _, a := range d.Applications {
		if a.FollowUp != "" && a.FollowUp <= today {
			followUps = append(followUps, map[string]interface{}{
				"id":        a.ID,
				"company":   a.Company,
				"role":      a.Role,
				"status":    a.Status,
				"follow_up": a.FollowUp,
				"notes":     a.Notes,
			})
		}
	}

	report := map[string]interface{}{
		"sync":         syncSection,
		"sweep":        sweepSection,
		"applications": applicationsSection,
		"metrics":      metricsSection,
		"follow_ups":   followUps,
	}

	reportJSON, err := json.Marshal(report)
	if err != nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("failed to build report: %v", err),
		}, nil
	}

	summary := fmt.Sprintf("Report: %d applications (%d active), %d follow-ups due, %d platforms synced",
		len(d.Applications), active, len(followUps), len(d.Sync.Results))

	return &soulv1.ToolResponse{
		Success:        true,
		Output:         summary,
		StructuredJson: string(reportJSON),
	}, nil
}

// ExecuteToolStream streams progress for streaming tools (sync, sweep);
// for all other tools it wraps ExecuteTool in a single Complete event.
func (s *ScoutService) ExecuteToolStream(req *soulv1.ToolRequest, stream grpc.ServerStreamingServer[soulv1.ToolEvent]) error {
	switch req.Tool {
	case "sync":
		return s.streamSync(req.InputJson, stream)
	case "sweep":
		return s.streamSweep(req.InputJson, stream)
	default:
		// For non-streaming tools, wrap ExecuteTool in a single Complete event.
		resp, err := s.ExecuteTool(context.Background(), req)
		if err != nil {
			return stream.Send(&soulv1.ToolEvent{
				Event: &soulv1.ToolEvent_Error{
					Error: &soulv1.ErrorEvent{
						Code:    "EXECUTE_ERROR",
						Message: err.Error(),
					},
				},
			})
		}

		return stream.Send(&soulv1.ToolEvent{
			Event: &soulv1.ToolEvent_Complete{
				Complete: resp,
			},
		})
	}
}

func (s *ScoutService) streamSync(inputJSON string, stream grpc.ServerStreamingServer[soulv1.ToolEvent]) error {
	var input syncInput
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return stream.Send(&soulv1.ToolEvent{
			Event: &soulv1.ToolEvent_Error{
				Error: &soulv1.ErrorEvent{
					Code:    "INVALID_INPUT",
					Message: fmt.Sprintf("invalid input: %v", err),
				},
			},
		})
	}

	// Expand "all" to all platforms plus website and github.
	platforms := input.Platforms
	for i, p := range platforms {
		if strings.ToLower(p) == "all" {
			platforms = append(browser.AllPlatforms(), "website", "github")
			_ = i
			break
		}
	}

	// Fetch profile data from Supabase.
	client, err := supabase.NewClient()
	if err != nil {
		return stream.Send(&soulv1.ToolEvent{
			Event: &soulv1.ToolEvent_Error{
				Error: &soulv1.ErrorEvent{
					Code:    "SUPABASE_ERROR",
					Message: fmt.Sprintf("supabase client error: %v", err),
				},
			},
		})
	}

	profile, err := client.GetProfileData()
	if err != nil {
		return stream.Send(&soulv1.ToolEvent{
			Event: &soulv1.ToolEvent_Error{
				Error: &soulv1.ErrorEvent{
					Code:    "PROFILE_ERROR",
					Message: fmt.Sprintf("fetch profile error: %v", err),
				},
			},
		})
	}

	var results []data.PlatformSync
	driftCount := 0

	for i, platform := range platforms {
		// Send progress.
		pct := float64(i) / float64(len(platforms)) * 100.0
		if err := stream.Send(&soulv1.ToolEvent{
			Event: &soulv1.ToolEvent_Progress{
				Progress: &soulv1.ProgressUpdate{
					Analyzer: platform,
					Percent:  pct,
					Message:  fmt.Sprintf("Checking %s...", platform),
				},
			},
		}); err != nil {
			return err
		}

		// Check the platform.
		result := syncpkg.CheckPlatform(platform, profile)
		results = append(results, result.Platform)

		// Send finding events for drift issues.
		if result.Platform.Status == "drift" {
			driftCount++
			for _, issue := range result.Platform.Issues {
				if err := stream.Send(&soulv1.ToolEvent{
					Event: &soulv1.ToolEvent_Finding{
						Finding: &soulv1.FindingEvent{
							Id:       fmt.Sprintf("sync-%s-%d", platform, i),
							Title:    fmt.Sprintf("Drift on %s", platform),
							Severity: "warning",
							Evidence: issue,
						},
					},
				}); err != nil {
					return err
				}
			}
		}
	}

	// Store results.
	if s.store != nil {
		if err := s.store.SetSyncResults(results); err != nil {
			log.Printf("[scout] warning: failed to save sync results: %v", err)
		}
	}

	// Send completion.
	syncedCount := len(results) - driftCount
	summary := fmt.Sprintf("Sync complete: %d/%d platforms in sync, %d with drift", syncedCount, len(results), driftCount)

	return stream.Send(&soulv1.ToolEvent{
		Event: &soulv1.ToolEvent_Complete{
			Complete: &soulv1.ToolResponse{
				Success: true,
				Output:  summary,
			},
		},
	})
}

func (s *ScoutService) streamSweep(inputJSON string, stream grpc.ServerStreamingServer[soulv1.ToolEvent]) error {
	var input sweepInput
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return stream.Send(&soulv1.ToolEvent{
			Event: &soulv1.ToolEvent_Error{
				Error: &soulv1.ErrorEvent{
					Code:    "INVALID_INPUT",
					Message: fmt.Sprintf("invalid input: %v", err),
				},
			},
		})
	}

	// Expand "all" to all platforms.
	platforms := input.Platforms
	for _, p := range platforms {
		if strings.ToLower(p) == "all" {
			platforms = browser.AllPlatforms()
			break
		}
	}

	// Fetch profile data for keyword/location defaults.
	client, err := supabase.NewClient()
	if err != nil {
		return stream.Send(&soulv1.ToolEvent{
			Event: &soulv1.ToolEvent_Error{
				Error: &soulv1.ErrorEvent{
					Code:    "SUPABASE_ERROR",
					Message: fmt.Sprintf("supabase client error: %v", err),
				},
			},
		})
	}

	profile, err := client.GetProfileData()
	if err != nil {
		return stream.Send(&soulv1.ToolEvent{
			Event: &soulv1.ToolEvent_Error{
				Error: &soulv1.ErrorEvent{
					Code:    "PROFILE_ERROR",
					Message: fmt.Sprintf("fetch profile error: %v", err),
				},
			},
		})
	}

	var allOpps []data.Opportunity
	var allMsgs []data.Message
	totalFound := 0

	for i, platform := range platforms {
		// Send progress.
		pct := float64(i) / float64(len(platforms)) * 100.0
		if err := stream.Send(&soulv1.ToolEvent{
			Event: &soulv1.ToolEvent_Progress{
				Progress: &soulv1.ProgressUpdate{
					Analyzer: platform,
					Percent:  pct,
					Message:  fmt.Sprintf("Sweeping %s for opportunities...", platform),
				},
			},
		}); err != nil {
			return err
		}

		// Sweep the platform.
		result := sweeppkg.SweepPlatform(platform, profile, input.Keywords, input.Location)
		if result.Error != nil {
			log.Printf("[scout] sweep %s error: %v", platform, result.Error)
			if err := stream.Send(&soulv1.ToolEvent{
				Event: &soulv1.ToolEvent_Finding{
					Finding: &soulv1.FindingEvent{
						Id:       fmt.Sprintf("sweep-err-%s", platform),
						Title:    fmt.Sprintf("Sweep error on %s", platform),
						Severity: "error",
						Evidence: result.Error.Error(),
					},
				},
			}); err != nil {
				return err
			}
			continue
		}

		allOpps = append(allOpps, result.Opportunities...)
		allMsgs = append(allMsgs, result.Messages...)

		for _, opp := range result.Opportunities {
			totalFound++
			if err := stream.Send(&soulv1.ToolEvent{
				Event: &soulv1.ToolEvent_Finding{
					Finding: &soulv1.FindingEvent{
						Id:       opp.ID,
						Title:    fmt.Sprintf("%s at %s", opp.Role, opp.Company),
						Severity: "info",
						File:     opp.URL,
						Evidence: fmt.Sprintf("Found on %s", platform),
					},
				},
			}); err != nil {
				return err
			}
		}
	}

	if s.store != nil {
		if err := s.store.SetSweepResults(allOpps, allMsgs); err != nil {
			log.Printf("[scout] warning: failed to save sweep results: %v", err)
		}
	}

	summary := fmt.Sprintf("Sweep complete: found %d opportunities across %d platforms", totalFound, len(platforms))

	return stream.Send(&soulv1.ToolEvent{
		Event: &soulv1.ToolEvent_Complete{
			Complete: &soulv1.ToolResponse{
				Success: true,
				Output:  summary,
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Profile tools
// ---------------------------------------------------------------------------

func (s *ScoutService) executeProfile(inputJSON string) (*soulv1.ToolResponse, error) {
	var input profileInput
	if inputJSON != "" {
		if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
			return &soulv1.ToolResponse{Success: false, Output: fmt.Sprintf("invalid input: %v", err)}, nil
		}
	}

	cfg, err := profiledb.LoadConfig()
	if err != nil {
		return &soulv1.ToolResponse{Success: false, Output: fmt.Sprintf("profile_db config: %v", err)}, nil
	}

	ctx := context.Background()
	client, err := profiledb.New(ctx, *cfg)
	if err != nil {
		return &soulv1.ToolResponse{Success: false, Output: fmt.Sprintf("connect profile_db: %v", err)}, nil
	}
	defer client.Close()

	var result map[string]json.RawMessage
	if len(input.Sections) > 0 {
		result, err = client.GetSections(ctx, input.Sections)
	} else {
		result, err = client.GetFullProfile(ctx)
	}
	if err != nil {
		return &soulv1.ToolResponse{Success: false, Output: fmt.Sprintf("fetch profile: %v", err)}, nil
	}

	jsonData, _ := json.Marshal(result)
	return &soulv1.ToolResponse{
		Success:        true,
		Output:         fmt.Sprintf("Profile loaded: %d sections", len(result)),
		StructuredJson: string(jsonData),
	}, nil
}

func (s *ScoutService) executeProfilePull(inputJSON string) (*soulv1.ToolResponse, error) {
	var input profilePullInput
	if inputJSON != "" {
		if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
			return &soulv1.ToolResponse{Success: false, Output: fmt.Sprintf("invalid input: %v", err)}, nil
		}
	}

	// Load Supabase config for the source.
	sbClient, err := supabase.NewClient()
	if err != nil {
		return &soulv1.ToolResponse{Success: false, Output: fmt.Sprintf("supabase config: %v", err)}, nil
	}

	// Load profile DB config for the destination.
	cfg, err := profiledb.LoadConfig()
	if err != nil {
		return &soulv1.ToolResponse{Success: false, Output: fmt.Sprintf("profile_db config: %v", err)}, nil
	}

	ctx := context.Background()
	client, err := profiledb.New(ctx, *cfg)
	if err != nil {
		return &soulv1.ToolResponse{Success: false, Output: fmt.Sprintf("connect profile_db: %v", err)}, nil
	}
	defer client.Close()

	var counts map[string]int
	if len(input.Tables) > 0 {
		counts = make(map[string]int, len(input.Tables))
		for _, table := range input.Tables {
			n, err := client.PullTable(ctx, sbClient.URL(), sbClient.AnonKey(), table)
			if err != nil {
				return &soulv1.ToolResponse{Success: false, Output: fmt.Sprintf("pull %s: %v", table, err)}, nil
			}
			counts[table] = n
		}
	} else {
		counts, err = client.PullAll(ctx, sbClient.URL(), sbClient.AnonKey())
		if err != nil {
			return &soulv1.ToolResponse{Success: false, Output: fmt.Sprintf("pull all: %v", err)}, nil
		}
	}

	// Build summary.
	var parts []string
	total := 0
	for _, table := range profiledb.AllTables {
		if n, ok := counts[table]; ok {
			parts = append(parts, fmt.Sprintf("%s(%d)", table, n))
			total += n
		}
	}
	summary := fmt.Sprintf("Pulled %d rows across %d tables: %s", total, len(counts), strings.Join(parts, ", "))
	log.Printf("[scout] profile_pull: %s", summary)

	return &soulv1.ToolResponse{
		Success: true,
		Output:  summary,
	}, nil
}

func (s *ScoutService) executeProfilePush(inputJSON string) (*soulv1.ToolResponse, error) {
	var input profilePushInput
	if inputJSON != "" {
		if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
			return &soulv1.ToolResponse{Success: false, Output: fmt.Sprintf("invalid input: %v", err)}, nil
		}
	}

	if !input.Confirm {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  "Push to Supabase requires confirm=true. This will overwrite live website data. Set confirm to true to proceed.",
		}, nil
	}

	// Load Supabase config for the destination.
	sbClient, err := supabase.NewClient()
	if err != nil {
		return &soulv1.ToolResponse{Success: false, Output: fmt.Sprintf("supabase config: %v", err)}, nil
	}

	// Load profile DB config for the source.
	cfg, err := profiledb.LoadConfig()
	if err != nil {
		return &soulv1.ToolResponse{Success: false, Output: fmt.Sprintf("profile_db config: %v", err)}, nil
	}

	ctx := context.Background()
	client, err := profiledb.New(ctx, *cfg)
	if err != nil {
		return &soulv1.ToolResponse{Success: false, Output: fmt.Sprintf("connect profile_db: %v", err)}, nil
	}
	defer client.Close()

	var counts map[string]int
	if len(input.Tables) > 0 {
		counts = make(map[string]int, len(input.Tables))
		for _, table := range input.Tables {
			n, err := client.PushTable(ctx, sbClient.URL(), sbClient.AnonKey(), table)
			if err != nil {
				return &soulv1.ToolResponse{Success: false, Output: fmt.Sprintf("push %s: %v", table, err)}, nil
			}
			counts[table] = n
		}
	} else {
		counts, err = client.PushAll(ctx, sbClient.URL(), sbClient.AnonKey())
		if err != nil {
			return &soulv1.ToolResponse{Success: false, Output: fmt.Sprintf("push all: %v", err)}, nil
		}
	}

	// Build summary.
	var parts []string
	total := 0
	for _, table := range profiledb.AllTables {
		if n, ok := counts[table]; ok {
			parts = append(parts, fmt.Sprintf("%s(%d)", table, n))
			total += n
		}
	}
	summary := fmt.Sprintf("Pushed %d rows across %d tables to Supabase: %s", total, len(counts), strings.Join(parts, ", "))
	log.Printf("[scout] profile_push: %s", summary)

	return &soulv1.ToolResponse{
		Success: true,
		Output:  summary,
	}, nil
}

// Health returns the health status of the scout service.
func (s *ScoutService) Health(_ context.Context, _ *soulv1.Empty) (*soulv1.HealthResponse, error) {
	return &soulv1.HealthResponse{
		Healthy: true,
		Status:  "ok",
	}, nil
}
