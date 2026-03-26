package sweep

import (
	"strings"

	"github.com/rishav1305/soul/internal/scout/store"
)

// FilterReason describes why a job was filtered out.
type FilterReason string

const (
	// FilterReasonCompanyTooSmall is returned when company_employee_count < 50
	// and is not 0 (0 means unknown).
	FilterReasonCompanyTooSmall FilterReason = "company_too_small"

	// FilterReasonNoLLMTech is returned when no LLM/GenAI technology slug is found.
	FilterReasonNoLLMTech FilterReason = "no_llm_tech"

	// FilterReasonStaffingExcluded is returned for staffing/recruiting or small
	// engineering services companies.
	FilterReasonStaffingExcluded FilterReason = "staffing_excluded"

	// FilterReasonDuplicateDomain is returned when a lead with the same
	// company_domain already exists in the active pipeline.
	FilterReasonDuplicateDomain FilterReason = "duplicate_domain"

	// FilterReasonSeniorityExcluded is returned for c_level, intern, or
	// entry_level seniority values.
	FilterReasonSeniorityExcluded FilterReason = "seniority_excluded"
)

// llmTechSlugs is the set of technology slugs that indicate LLM/GenAI relevance.
var llmTechSlugs = map[string]bool{
	"langchain":   true,
	"langgraph":   true,
	"claude":      true,
	"anthropic":   true,
	"openai":      true,
	"llm":         true,
	"rag":         true,
	"vector":      true,
	"embeddings":  true,
	"transformers": true,
	"hugging-face": true,
}

// ShouldFilter reports whether a Job should be excluded from the pipeline.
// It returns (true, reason) if the job is filtered, or (false, "") if it passes.
//
// Filters applied in order:
//  1. Minimum company size: skip if employee_count < 50 (0 = unknown, not filtered).
//  2. LLM/GenAI tech: require at least one matching technology slug.
//  3. Staffing/recruiting exclusion: skip "Staffing and Recruiting"; skip
//     "Engineering Services" if employee_count < 100 (and count > 0).
//  4. Duplicate domain: skip if another active lead shares the company_domain.
//  5. Seniority: skip c_level, intern (or containing "intern"), entry_level.
func ShouldFilter(job Job, st *store.Store) (bool, FilterReason) {
	// --- Filter 1: Minimum Company Size ---
	if job.CompanyObject != nil {
		count := job.CompanyObject.EmployeeCount
		if count != 0 && count < 50 {
			return true, FilterReasonCompanyTooSmall
		}
	}

	// --- Filter 2: LLM/GenAI Technology ---
	hasLLMTech := false
	for _, slug := range job.TechnologySlugs {
		if llmTechSlugs[slug] {
			hasLLMTech = true
			break
		}
	}
	if !hasLLMTech {
		return true, FilterReasonNoLLMTech
	}

	// --- Filter 3: Staffing/Recruiting Exclusion ---
	if job.CompanyObject != nil {
		industry := job.CompanyObject.Industry
		if industry == "Staffing and Recruiting" {
			return true, FilterReasonStaffingExcluded
		}
		if industry == "Engineering Services" {
			count := job.CompanyObject.EmployeeCount
			// Only filter if count is known (non-zero) and below 100.
			if count != 0 && count < 100 {
				return true, FilterReasonStaffingExcluded
			}
		}
	}

	// --- Filter 4: Duplicate Company Domain ---
	// Skip if a different active lead already has this company_domain.
	// Leads with the same theirstack_id are handled by AddLeadIfNotExists, not here.
	domain := job.CompanyDomain
	if domain == "" && job.CompanyObject != nil {
		domain = job.CompanyObject.Domain
	}
	if domain != "" {
		var exists int
		_ = st.DB().QueryRow(
			"SELECT COUNT(*) FROM leads WHERE company_domain = ? AND closed_at = '' AND (theirstack_id IS NULL OR theirstack_id != ?)",
			domain, job.ID,
		).Scan(&exists)
		if exists > 0 {
			return true, FilterReasonDuplicateDomain
		}
	}

	// --- Filter 5: Role Seniority ---
	seniority := job.Seniority
	if seniority == "c_level" ||
		seniority == "entry_level" ||
		strings.Contains(seniority, "intern") {
		return true, FilterReasonSeniorityExcluded
	}

	return false, ""
}
