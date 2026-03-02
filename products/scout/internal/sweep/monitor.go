// Package sweep provides job opportunity extraction from platform pages
// using headless Chrome automation. It navigates to each platform's jobs
// URL and extracts job cards from the DOM.
package sweep

import (
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"

	"github.com/rishav1305/soul/products/scout/internal/browser"
	"github.com/rishav1305/soul/products/scout/internal/data"
	"github.com/rishav1305/soul/products/scout/internal/supabase"
)

// SweepResult holds the outcome of sweeping a single platform.
type SweepResult struct {
	Opportunities []data.Opportunity
	Messages      []data.Message
	Error         error
}

// SweepPlatform extracts job opportunities from a single platform.
// It accepts profile data, keywords, and location for constructing search URLs.
// If keywords are empty, title keywords from the Supabase profile are used.
func SweepPlatform(platform string, profile *supabase.ProfileData, keywords []string, location string) SweepResult {
	if _, ok := browser.PlatformURLs[platform]; !ok {
		return SweepResult{
			Error: fmt.Errorf("unknown platform: %s", platform),
		}
	}

	if !browser.HasProfile(platform) {
		return SweepResult{
			Error: fmt.Errorf("no browser profile for %s — run setup first", platform),
		}
	}

	// Derive keywords from profile title if not provided.
	// Use only the first segment to avoid overly specific queries.
	if len(keywords) == 0 && profile != nil && len(profile.SiteConfig) > 0 {
		parts := splitTitleKeywords(profile.SiteConfig[0].Title)
		if len(parts) > 0 {
			keywords = []string{parts[0]}
		}
	}

	// Derive location from profile if not provided.
	if location == "" && profile != nil && len(profile.SiteConfig) > 0 {
		location = profile.SiteConfig[0].Location
	}

	// LinkedIn public job search doesn't need authentication.
	// Use a clean browser to avoid authwall detection.
	var (
		b   *rod.Browser
		err error
	)
	if platform == "linkedin" {
		b, err = browser.LaunchHeadlessNoPlatform()
	} else {
		b, err = browser.LaunchHeadless(platform)
	}
	if err != nil {
		return SweepResult{Error: fmt.Errorf("launch chrome: %w", err)}
	}
	defer b.MustClose()

	page, err := b.Page(proto.TargetCreateTarget{})
	if err != nil {
		return SweepResult{Error: fmt.Errorf("create page: %w", err)}
	}

	switch platform {
	case "linkedin":
		return sweepLinkedIn(page, platform, keywords, location)
	case "naukri":
		return sweepNaukri(page, platform, keywords, location)
	case "indeed":
		return sweepIndeed(page, platform, keywords, location)
	case "wellfound":
		return sweepWellfound(page, platform, keywords, location)
	case "instahyre":
		return sweepInstahyre(page, platform, keywords, location)
	default:
		return SweepResult{}
	}
}

// sweepLinkedIn searches LinkedIn Jobs with keyword-based search URL.
func sweepLinkedIn(page *rod.Page, platform string, keywords []string, location string) SweepResult {
	var opps []data.Opportunity
	now := time.Now().Format(time.RFC3339)

	// Build LinkedIn job search URL with keywords and location.
	searchURL := buildLinkedInSearchURL(keywords, location)
	log.Printf("[scout] sweep linkedin: %s", searchURL)

	if err := browser.NavigateWithDelay(page, searchURL); err != nil {
		return SweepResult{Error: fmt.Errorf("navigate to LinkedIn jobs search: %w", err)}
	}

	// Wait for job results to load — try multiple selectors.
	jobListSelectors := []string{
		".jobs-search__results-list",
		".jobs-search-results-list",
		".scaffold-layout__list",
		"ul.jobs-search__results-list",
	}

	var listFound bool
	for _, sel := range jobListSelectors {
		el, err := page.Timeout(10 * time.Second).Element(sel)
		if err == nil && el != nil {
			listFound = true
			log.Printf("[scout] sweep linkedin: found job list with selector %q", sel)
			break
		}
	}

	if !listFound {
		log.Printf("[scout] sweep linkedin: no job list container found, trying individual cards")
	}

	// Try multiple card selectors — LinkedIn changes DOM frequently.
	cardSelectors := []string{
		".jobs-search-results__list-item",
		".job-card-container",
		".scaffold-layout__list-item",
		".job-card-list",
		"li.jobs-search-results__list-item",
		".base-search-card",
	}

	var elements rod.Elements
	for _, sel := range cardSelectors {
		els, err := page.Elements(sel)
		if err == nil && len(els) > 0 {
			elements = els
			log.Printf("[scout] sweep linkedin: found %d cards with selector %q", len(els), sel)
			break
		}
	}

	// Fallback: extract from links if no card selectors work.
	if len(elements) == 0 {
		log.Printf("[scout] sweep linkedin: card selectors failed, falling back to link extraction")
		return sweepLinkedInFallback(page, platform, now)
	}

	for i, el := range elements {
		if i >= 15 {
			break
		}

		var role, company, jobURL, loc, posted string

		// Try multiple title selectors.
		titleSelectors := []string{
			".job-card-list__title",
			".base-search-card__title",
			".artdeco-entity-lockup__title",
			"a.job-card-container__link",
			"h3",
		}
		for _, sel := range titleSelectors {
			titleEl, err := el.Element(sel)
			if err == nil {
				role, _ = titleEl.Text()
				role = strings.TrimSpace(role)
				if role != "" {
					break
				}
			}
		}

		// Try multiple company selectors.
		companySelectors := []string{
			".job-card-container__primary-description",
			".base-search-card__subtitle",
			".artdeco-entity-lockup__subtitle",
			"h4",
		}
		for _, sel := range companySelectors {
			companyEl, err := el.Element(sel)
			if err == nil {
				company, _ = companyEl.Text()
				company = strings.TrimSpace(company)
				if company != "" {
					break
				}
			}
		}

		// Extract URL from first link.
		linkEl, err := el.Element("a")
		if err == nil {
			if href, err := linkEl.Attribute("href"); err == nil && href != nil {
				jobURL = *href
				if jobURL != "" && jobURL[0] == '/' {
					jobURL = "https://www.linkedin.com" + jobURL
				}
			}
		}

		// Try to extract location.
		locSelectors := []string{
			".job-card-container__metadata-item",
			".base-search-card__metadata",
			".job-card-container__metadata-wrapper",
		}
		for _, sel := range locSelectors {
			locEl, err := el.Element(sel)
			if err == nil {
				loc, _ = locEl.Text()
				loc = strings.TrimSpace(loc)
				if loc != "" {
					break
				}
			}
		}

		// Try to extract posted date.
		timeSelectors := []string{
			"time",
			".job-card-container__listed-time",
			".base-search-card__listing-date",
		}
		for _, sel := range timeSelectors {
			timeEl, err := el.Element(sel)
			if err == nil {
				posted, _ = timeEl.Text()
				posted = strings.TrimSpace(posted)
				if posted != "" {
					break
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
				URL:      jobURL,
				Location: loc,
				PostedAt: posted,
				FoundAt:  now,
			})
		}
	}

	return SweepResult{Opportunities: opps}
}

// sweepLinkedInFallback extracts job opportunities from links on the page
// when card-based selectors don't work.
func sweepLinkedInFallback(page *rod.Page, platform, now string) SweepResult {
	var opps []data.Opportunity

	// Find all links that look like job postings.
	links, err := page.Elements("a[href*='/jobs/view/'], a[href*='/jobs/collections/']")
	if err != nil || len(links) == 0 {
		log.Printf("[scout] sweep linkedin fallback: no job links found")
		return SweepResult{Opportunities: opps}
	}

	seen := make(map[string]bool)
	for i, link := range links {
		if i >= 20 {
			break
		}

		href, err := link.Attribute("href")
		if err != nil || href == nil || *href == "" {
			continue
		}

		jobURL := *href
		if jobURL[0] == '/' {
			jobURL = "https://www.linkedin.com" + jobURL
		}

		// Deduplicate by URL.
		if seen[jobURL] {
			continue
		}
		seen[jobURL] = true

		text, _ := link.Text()
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}

		opps = append(opps, data.Opportunity{
			ID:       fmt.Sprintf("li-%d-%d", time.Now().UnixMilli(), len(opps)),
			Role:     text,
			Platform: platform,
			URL:      jobURL,
			FoundAt:  now,
		})

		if len(opps) >= 15 {
			break
		}
	}

	log.Printf("[scout] sweep linkedin fallback: found %d opportunities from links", len(opps))
	return SweepResult{Opportunities: opps}
}

// buildLinkedInSearchURL constructs a public (no-auth) LinkedIn job search URL.
// The public jobs search page at /jobs/search works without login.
func buildLinkedInSearchURL(keywords []string, location string) string {
	kw := strings.Join(keywords, " ")
	if kw == "" {
		kw = "AI Engineer"
	}

	params := url.Values{}
	params.Set("keywords", kw)
	if location != "" {
		params.Set("location", location)
	}
	params.Set("sortBy", "DD")
	params.Set("position", "1")
	params.Set("pageNum", "0")

	// Use the guest-accessible jobs search page.
	return "https://www.linkedin.com/jobs/search?" + params.Encode()
}

// sweepNaukri extracts job cards from Naukri's jobs page.
func sweepNaukri(page *rod.Page, platform string, keywords []string, location string) SweepResult {
	_ = keywords
	_ = location

	urls := browser.PlatformURLs[platform]
	if err := browser.NavigateWithDelay(page, urls.Jobs); err != nil {
		return SweepResult{Error: fmt.Errorf("navigate to Naukri: %w", err)}
	}

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

		var role, company, jobURL string

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
				jobURL = *href
			}
		}

		if role != "" || company != "" {
			opps = append(opps, data.Opportunity{
				ID:       fmt.Sprintf("nk-%d-%d", time.Now().UnixMilli(), i),
				Company:  company,
				Role:     role,
				Platform: platform,
				Match:    0,
				URL:      jobURL,
				FoundAt:  now,
			})
		}
	}

	return SweepResult{Opportunities: opps}
}

// sweepIndeed returns empty results — Indeed requires more complex handling.
func sweepIndeed(_ *rod.Page, _ string, _ []string, _ string) SweepResult {
	return SweepResult{
		Opportunities: []data.Opportunity{},
		Messages:      []data.Message{},
	}
}

// sweepWellfound returns empty results — Wellfound's SPA requires more complex handling.
func sweepWellfound(_ *rod.Page, _ string, _ []string, _ string) SweepResult {
	return SweepResult{
		Opportunities: []data.Opportunity{},
		Messages:      []data.Message{},
	}
}

// sweepInstahyre returns empty results — Instahyre requires more complex handling.
func sweepInstahyre(_ *rod.Page, _ string, _ []string, _ string) SweepResult {
	return SweepResult{
		Opportunities: []data.Opportunity{},
		Messages:      []data.Message{},
	}
}

// splitTitleKeywords splits a title like "AI Engineer | AI Consultant" into segments.
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
