// Package sweep provides job opportunity extraction from platform pages
// using headless Chrome automation. It navigates to each platform's jobs
// URL and extracts job cards from the DOM.
package sweep

import (
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"

	"github.com/rishav1305/soul/products/scout/internal/browser"
	"github.com/rishav1305/soul/products/scout/internal/data"
)

// SweepResult holds the outcome of sweeping a single platform.
type SweepResult struct {
	Opportunities []data.Opportunity
	Messages      []data.Message
	Error         error
}

// SweepPlatform extracts job opportunities from a single platform.
// It launches a headless browser, navigates to the platform's Jobs URL,
// and extracts job cards using platform-specific DOM selectors.
// Results are limited to 10 per platform.
func SweepPlatform(platform string) SweepResult {
	urls, ok := browser.PlatformURLs[platform]
	if !ok {
		return SweepResult{
			Error: fmt.Errorf("unknown platform: %s", platform),
		}
	}

	if !browser.HasProfile(platform) {
		return SweepResult{
			Error: fmt.Errorf("no browser profile for %s — run setup first", platform),
		}
	}

	profileDir, err := browser.ProfileDir(platform)
	if err != nil {
		return SweepResult{Error: fmt.Errorf("profile dir: %w", err)}
	}

	u, err := launcher.New().
		UserDataDir(profileDir).
		Headless(true).
		Launch()
	if err != nil {
		return SweepResult{Error: fmt.Errorf("launch chrome: %w", err)}
	}

	b := rod.New().ControlURL(u)
	if err := b.Connect(); err != nil {
		return SweepResult{Error: fmt.Errorf("connect chrome: %w", err)}
	}
	defer b.MustClose()

	page, err := b.Page(proto.TargetCreateTarget{})
	if err != nil {
		return SweepResult{Error: fmt.Errorf("create page: %w", err)}
	}

	if err := browser.NavigateWithDelay(page, urls.Jobs); err != nil {
		return SweepResult{Error: fmt.Errorf("navigate: %w", err)}
	}

	switch platform {
	case "linkedin":
		return sweepLinkedIn(page, platform)
	case "naukri":
		return sweepNaukri(page, platform)
	case "indeed":
		return sweepIndeed(page, platform)
	case "wellfound":
		return sweepWellfound(page, platform)
	case "instahyre":
		return sweepInstahyre(page, platform)
	default:
		return SweepResult{}
	}
}

// sweepLinkedIn extracts job cards from LinkedIn's jobs page.
func sweepLinkedIn(page *rod.Page, platform string) SweepResult {
	var opps []data.Opportunity
	now := time.Now().Format(time.RFC3339)

	// LinkedIn job cards use various selectors depending on feed vs search.
	elements, err := page.Elements(".job-card-container, .jobs-search-results__list-item, .scaffold-layout__list-item")
	if err != nil {
		return SweepResult{Error: fmt.Errorf("query LinkedIn job cards: %w", err)}
	}

	for i, el := range elements {
		if i >= 10 {
			break
		}

		var role, company, url string

		titleEl, err := el.Element(".job-card-list__title, .artdeco-entity-lockup__title")
		if err == nil {
			role, _ = titleEl.Text()
		}

		companyEl, err := el.Element(".job-card-container__primary-description, .artdeco-entity-lockup__subtitle")
		if err == nil {
			company, _ = companyEl.Text()
		}

		linkEl, err := el.Element("a")
		if err == nil {
			if href, err := linkEl.Attribute("href"); err == nil && href != nil {
				url = *href
				if url != "" && url[0] == '/' {
					url = "https://www.linkedin.com" + url
				}
			}
		}

		if role != "" || company != "" {
			opps = append(opps, data.Opportunity{
				ID:       fmt.Sprintf("li-%d-%d", time.Now().UnixMilli(), i),
				Company:  company,
				Role:     role,
				Platform: platform,
				Match:    0,
				URL:      url,
				FoundAt:  now,
			})
		}
	}

	return SweepResult{Opportunities: opps}
}

// sweepNaukri extracts job cards from Naukri's jobs page.
func sweepNaukri(page *rod.Page, platform string) SweepResult {
	var opps []data.Opportunity
	now := time.Now().Format(time.RFC3339)

	elements, err := page.Elements(".jobTuple, .srp-jobtuple-wrapper, .cust-job-tuple")
	if err != nil {
		return SweepResult{Error: fmt.Errorf("query Naukri job cards: %w", err)}
	}

	for i, el := range elements {
		if i >= 10 {
			break
		}

		var role, company, url string

		titleEl, err := el.Element(".title, .desig")
		if err == nil {
			role, _ = titleEl.Text()
		}

		companyEl, err := el.Element(".comp-name, .companyInfo .subTitle")
		if err == nil {
			company, _ = companyEl.Text()
		}

		linkEl, err := el.Element("a.title, a")
		if err == nil {
			if href, err := linkEl.Attribute("href"); err == nil && href != nil {
				url = *href
			}
		}

		if role != "" || company != "" {
			opps = append(opps, data.Opportunity{
				ID:       fmt.Sprintf("nk-%d-%d", time.Now().UnixMilli(), i),
				Company:  company,
				Role:     role,
				Platform: platform,
				Match:    0,
				URL:      url,
				FoundAt:  now,
			})
		}
	}

	return SweepResult{Opportunities: opps}
}

// sweepIndeed extracts job cards from Indeed. Returns empty results as
// Indeed's DOM structure requires more complex handling.
func sweepIndeed(page *rod.Page, platform string) SweepResult {
	_ = page
	_ = platform
	return SweepResult{
		Opportunities: []data.Opportunity{},
		Messages:      []data.Message{},
	}
}

// sweepWellfound extracts job cards from Wellfound. Returns empty results
// as Wellfound's SPA requires more complex handling.
func sweepWellfound(page *rod.Page, platform string) SweepResult {
	_ = page
	_ = platform
	return SweepResult{
		Opportunities: []data.Opportunity{},
		Messages:      []data.Message{},
	}
}

// sweepInstahyre extracts job cards from Instahyre. Returns empty results
// as Instahyre's DOM structure requires more complex handling.
func sweepInstahyre(page *rod.Page, platform string) SweepResult {
	_ = page
	_ = platform
	return SweepResult{
		Opportunities: []data.Opportunity{},
		Messages:      []data.Message{},
	}
}
