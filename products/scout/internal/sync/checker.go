// Package sync provides platform profile comparison checking.
// It compares live platform profiles against the Supabase source of truth,
// reporting drift where profiles are out of date.
package sync

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
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
// It navigates to the platform's Jobs URL and checks if the user's title
// from site_config appears on the page.
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

	// Extract the title from site_config.
	var title string
	if len(profile.SiteConfig) > 0 {
		title = profile.SiteConfig[0].Title
	}

	// Launch headless browser with existing profile.
	profileDir, err := browser.ProfileDir(platform)
	if err != nil {
		return CheckResult{
			Platform: data.PlatformSync{
				Platform:  platform,
				Status:    "error",
				Issues:    []string{fmt.Sprintf("profile dir error: %v", err)},
				CheckedAt: now,
			},
			Error: err,
		}
	}

	u, err := launcher.New().
		UserDataDir(profileDir).
		Headless(true).
		Launch()
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

	b := rod.New().ControlURL(u)
	if err := b.Connect(); err != nil {
		return CheckResult{
			Platform: data.PlatformSync{
				Platform:  platform,
				Status:    "error",
				Issues:    []string{fmt.Sprintf("chrome connect error: %v", err)},
				CheckedAt: now,
			},
			Error: err,
		}
	}
	defer b.MustClose()

	// Create a new page and navigate to the jobs URL.
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

	if err := browser.NavigateWithDelay(page, urls.Jobs); err != nil {
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

	// Get the page text and check for the user's title.
	var issues []string
	el, err := page.Element("body")
	if err == nil {
		text, err := el.Text()
		if err == nil && title != "" {
			if !strings.Contains(strings.ToLower(text), strings.ToLower(title)) {
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
			CheckedAt: now,
		},
	}
}
