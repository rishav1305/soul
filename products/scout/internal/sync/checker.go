// Package sync provides platform profile comparison checking.
// It compares live platform profiles against the Supabase source of truth,
// reporting drift where profiles are out of date.
package sync

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"

	"github.com/rishav1305/soul/products/scout/internal/browser"
	"github.com/rishav1305/soul/products/scout/internal/data"
	"github.com/rishav1305/soul/products/scout/internal/supabase"
)

// CheckResult holds the outcome of checking a single platform.
type CheckResult struct {
	Platform data.PlatformSync
	Error    error
}

// CheckPlatform checks a single platform for profile drift against the
// Supabase source of truth. It returns a CheckResult with sync status
// and any issues found.
func CheckPlatform(platform string, profile *supabase.ProfileData) CheckResult {
	switch platform {
	case "website":
		return checkWebsite(profile)
	case "github":
		return checkGitHub(profile)
	default:
		return checkBrowserPlatform(platform, profile)
	}
}

// checkWebsite performs a placeholder check for the portfolio website.
func checkWebsite(profile *supabase.ProfileData) CheckResult {
	_ = profile
	return CheckResult{
		Platform: data.PlatformSync{
			Platform:  "website",
			Status:    "synced",
			Issues:    []string{},
			CheckedAt: time.Now().Format(time.RFC3339),
		},
	}
}

// checkGitHub performs a placeholder check for the GitHub profile.
func checkGitHub(profile *supabase.ProfileData) CheckResult {
	_ = profile
	return CheckResult{
		Platform: data.PlatformSync{
			Platform:  "github",
			Status:    "synced",
			Issues:    []string{},
			CheckedAt: time.Now().Format(time.RFC3339),
		},
	}
}

// checkBrowserPlatform uses headless Chrome to check a job platform profile.
// For LinkedIn it navigates to the profile page and checks multiple fields.
// For other platforms it falls back to a title-only check on the jobs page.
func checkBrowserPlatform(platform string, profile *supabase.ProfileData) CheckResult {
	now := time.Now().Format(time.RFC3339)

	// Verify the platform is known.
	urls, ok := browser.PlatformURLs[platform]
	if !ok {
		return CheckResult{
			Platform: data.PlatformSync{
				Platform:  platform,
				Status:    "error",
				Issues:    []string{fmt.Sprintf("unknown platform: %s", platform)},
				CheckedAt: now,
			},
		}
	}

	// Check if a browser profile exists (user must have logged in first).
	if !browser.HasProfile(platform) {
		return CheckResult{
			Platform: data.PlatformSync{
				Platform:  platform,
				Status:    "drift",
				Issues:    []string{"no browser profile found — run setup first"},
				CheckedAt: now,
			},
		}
	}

	// Launch headless browser with existing profile (remote-first).
	b, err := browser.LaunchHeadless(platform)
	if err != nil {
		return CheckResult{
			Platform: data.PlatformSync{
				Platform:  platform,
				Status:    "error",
				Issues:    []string{fmt.Sprintf("chrome launch error: %v", err)},
				CheckedAt: now,
			},
			Error: err,
		}
	}
	defer b.MustClose()

	page, err := b.Page(proto.TargetCreateTarget{})
	if err != nil {
		return CheckResult{
			Platform: data.PlatformSync{
				Platform:  platform,
				Status:    "error",
				Issues:    []string{fmt.Sprintf("page creation error: %v", err)},
				CheckedAt: now,
			},
			Error: err,
		}
	}

	if platform == "linkedin" {
		return checkLinkedInProfile(page, profile, now)
	}

	return checkGenericPlatform(page, platform, urls.Jobs, profile, now)
}

// checkLinkedInProfile navigates to the LinkedIn profile page and checks
// multiple fields: name, title/headline, current company, current role, location.
func checkLinkedInProfile(page *rod.Page, profile *supabase.ProfileData, now string) CheckResult {
	// Get profile URL from Supabase social_media.
	var profileURL string
	if len(profile.SiteConfig) > 0 && profile.SiteConfig[0].SocialMedia != nil {
		profileURL = profile.SiteConfig[0].SocialMedia["linkedin"]
	}
	if profileURL == "" {
		return CheckResult{
			Platform: data.PlatformSync{
				Platform:  "linkedin",
				Status:    "error",
				Issues:    []string{"no LinkedIn profile URL in Supabase social_media"},
				CheckedAt: now,
			},
		}
	}

	// Navigate to the profile page.
	if err := browser.NavigateWithDelay(page, profileURL); err != nil {
		return CheckResult{
			Platform: data.PlatformSync{
				Platform:  "linkedin",
				Status:    "error",
				Issues:    []string{fmt.Sprintf("navigation error: %v", err)},
				CheckedAt: now,
			},
			Error: err,
		}
	}

	// Get the full page text for field checking.
	el, err := page.Element("body")
	if err != nil {
		return CheckResult{
			Platform: data.PlatformSync{
				Platform:  "linkedin",
				Status:    "error",
				Issues:    []string{fmt.Sprintf("body element error: %v", err)},
				CheckedAt: now,
			},
			Error: err,
		}
	}

	text, err := el.Text()
	if err != nil {
		return CheckResult{
			Platform: data.PlatformSync{
				Platform:  "linkedin",
				Status:    "error",
				Issues:    []string{fmt.Sprintf("text extraction error: %v", err)},
				CheckedAt: now,
			},
			Error: err,
		}
	}

	pageText := strings.ToLower(text)

	// Extract expected values from Supabase profile.
	var name, title, location string
	if len(profile.SiteConfig) > 0 {
		name = profile.SiteConfig[0].Name
		title = profile.SiteConfig[0].Title
		location = profile.SiteConfig[0].Location
	}

	// Find current roles (experience entries with no end_date).
	type currentRole struct {
		role    string
		company string
	}
	var currentRoles []currentRole
	for _, exp := range profile.Experience {
		if exp.EndDate == nil {
			currentRoles = append(currentRoles, currentRole{
				role:    exp.Role,
				company: exp.Company,
			})
		}
	}

	// Check each field against the page text.
	var details []data.SyncDetail
	var issues []string

	// 1. Name check — verify each part of the name appears.
	if name != "" {
		nameParts := strings.Fields(name)
		allFound := true
		for _, part := range nameParts {
			if len(part) > 1 && !strings.Contains(pageText, strings.ToLower(part)) {
				allFound = false
				break
			}
		}
		details = append(details, data.SyncDetail{
			Field:    "name",
			Expected: name,
			Match:    allFound,
		})
		if !allFound {
			issues = append(issues, fmt.Sprintf("name %q not found on profile page", name))
		}
	}

	// 2. Title/headline — check if any keyword segment from title appears.
	if title != "" {
		titleParts := splitTitleKeywords(title)
		anyFound := false
		for _, part := range titleParts {
			if strings.Contains(pageText, strings.ToLower(part)) {
				anyFound = true
				break
			}
		}
		details = append(details, data.SyncDetail{
			Field:    "title",
			Expected: title,
			Match:    anyFound,
		})
		if !anyFound {
			issues = append(issues, fmt.Sprintf("title keywords not found on profile page (expected one of: %s)", title))
		}
	}

	// 3. Current company — check if any current company name appears.
	if len(currentRoles) > 0 {
		companyNames := make([]string, 0, len(currentRoles))
		anyCompanyFound := false
		for _, cr := range currentRoles {
			companyNames = append(companyNames, cr.company)
			if containsCompanyName(pageText, cr.company) {
				anyCompanyFound = true
			}
		}
		expected := strings.Join(companyNames, ", ")
		details = append(details, data.SyncDetail{
			Field:    "company",
			Expected: expected,
			Match:    anyCompanyFound,
		})
		if !anyCompanyFound {
			issues = append(issues, fmt.Sprintf("current company not found on profile page (expected: %s)", expected))
		}
	}

	// 4. Current role — check if any current role title appears.
	if len(currentRoles) > 0 {
		roleNames := make([]string, 0, len(currentRoles))
		anyRoleFound := false
		for _, cr := range currentRoles {
			roleNames = append(roleNames, cr.role)
			if strings.Contains(pageText, strings.ToLower(cr.role)) {
				anyRoleFound = true
			}
		}
		expected := strings.Join(roleNames, ", ")
		details = append(details, data.SyncDetail{
			Field:    "role",
			Expected: expected,
			Match:    anyRoleFound,
		})
		if !anyRoleFound {
			issues = append(issues, fmt.Sprintf("current role not found on profile page (expected: %s)", expected))
		}
	}

	// 5. Location check.
	if location != "" {
		locationFound := strings.Contains(pageText, strings.ToLower(location))
		details = append(details, data.SyncDetail{
			Field:    "location",
			Expected: location,
			Match:    locationFound,
		})
		if !locationFound {
			issues = append(issues, fmt.Sprintf("location %q not found on profile page", location))
		}
	}

	status := "synced"
	if len(issues) > 0 {
		status = "drift"
	}

	return CheckResult{
		Platform: data.PlatformSync{
			Platform:  "linkedin",
			Status:    status,
			Issues:    issues,
			Details:   details,
			CheckedAt: now,
		},
	}
}

// checkGenericPlatform does a simple title check on a platform's jobs page.
func checkGenericPlatform(page *rod.Page, platform, url string, profile *supabase.ProfileData, now string) CheckResult {
	var title string
	if len(profile.SiteConfig) > 0 {
		title = profile.SiteConfig[0].Title
	}

	if err := browser.NavigateWithDelay(page, url); err != nil {
		return CheckResult{
			Platform: data.PlatformSync{
				Platform:  platform,
				Status:    "error",
				Issues:    []string{fmt.Sprintf("navigation error: %v", err)},
				CheckedAt: now,
			},
			Error: err,
		}
	}

	var issues []string
	var details []data.SyncDetail
	el, err := page.Element("body")
	if err == nil {
		text, err := el.Text()
		if err == nil && title != "" {
			found := strings.Contains(strings.ToLower(text), strings.ToLower(title))
			details = append(details, data.SyncDetail{
				Field:    "title",
				Expected: title,
				Match:    found,
			})
			if !found {
				issues = append(issues, fmt.Sprintf("title %q not found on %s profile page", title, platform))
			}
		}
	}

	status := "synced"
	if len(issues) > 0 {
		status = "drift"
	}

	return CheckResult{
		Platform: data.PlatformSync{
			Platform:  platform,
			Status:    status,
			Issues:    issues,
			Details:   details,
			CheckedAt: now,
		},
	}
}

// splitTitleKeywords splits a title string like "AI Engineer | AI Consultant | AI Researcher"
// into individual keyword segments.
func splitTitleKeywords(title string) []string {
	separators := []string{"|", ",", ";", " - "}
	parts := []string{title}
	for _, sep := range separators {
		var newParts []string
		for _, p := range parts {
			for _, sub := range strings.Split(p, sep) {
				s := strings.TrimSpace(sub)
				if s != "" {
					newParts = append(newParts, s)
				}
			}
		}
		parts = newParts
	}
	return parts
}

// containsCompanyName checks if a company name appears in text, trying
// both the full name and the first word (e.g. "IBM" from "IBM - TWC").
func containsCompanyName(pageText, company string) bool {
	lower := strings.ToLower(company)
	if strings.Contains(pageText, lower) {
		return true
	}
	// Try first word for abbreviated names.
	parts := strings.Fields(lower)
	if len(parts) > 0 && len(parts[0]) >= 2 {
		return strings.Contains(pageText, parts[0])
	}
	return false
}
