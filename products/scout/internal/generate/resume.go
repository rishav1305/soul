package generate

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"

	"github.com/rishav1305/soul/products/scout/internal/supabase"
	"github.com/rishav1305/soul/products/scout/templates"
)

// ResumeData is the template data struct for resume rendering.
type ResumeData struct {
	Name       string
	Headline   string
	Email      string
	Website    string
	Location   string
	Summary    string
	Experience []ExperienceEntry
	Skills     []SkillEntry
	Projects   []ProjectEntry
}

// ExperienceEntry represents a single work experience block.
type ExperienceEntry struct {
	Role         string
	Company      string
	Period       string
	Achievements []string
}

// SkillEntry represents a single skill with optional emphasis.
type SkillEntry struct {
	Name     string
	Emphasis bool
}

// ProjectEntry represents a portfolio project.
type ProjectEntry struct {
	Title       string
	Description string
}

// CoverData is the template data struct for cover letter rendering.
type CoverData struct {
	Date      string
	Company   string
	CoverText string
	Name      string
	Email     string
	Website   string
}

// profileConfig extracts the first site_config row into simple field accessors.
func profileConfig(profile *supabase.ProfileData) (name, email, website, location string) {
	if len(profile.SiteConfig) == 0 {
		return
	}
	sc := profile.SiteConfig[0]
	name = sc.Name
	email = sc.Email
	location = sc.Location
	if linkedin, ok := sc.SocialMedia["linkedin"]; ok {
		website = linkedin
	}
	return
}

// BuildResumeHTML renders the resume HTML template using profile data
// tailored to the given variant.
func BuildResumeHTML(variant Variant, profile *supabase.ProfileData) (string, error) {
	name, email, website, location := profileConfig(profile)

	// Build skills list with emphasis skills first.
	// Flatten skill_categories into individual skill entries.
	emphasisSet := make(map[string]bool, len(variant.SkillEmphasis))
	for _, s := range variant.SkillEmphasis {
		emphasisSet[strings.ToLower(s)] = true
	}

	var emphasized []SkillEntry
	var regular []SkillEntry
	seen := make(map[string]bool)
	for _, cat := range profile.Skills {
		for _, sk := range cat.Skills {
			if seen[strings.ToLower(sk.Name)] {
				continue
			}
			seen[strings.ToLower(sk.Name)] = true
			entry := SkillEntry{Name: sk.Name}
			if emphasisSet[strings.ToLower(sk.Name)] || emphasisSet[strings.ToLower(cat.CategoryName)] {
				entry.Emphasis = true
				emphasized = append(emphasized, entry)
			} else {
				regular = append(regular, entry)
			}
		}
	}
	// Also add variant emphasis skills that may not be in the profile.
	for _, s := range variant.SkillEmphasis {
		if !seen[strings.ToLower(s)] {
			emphasized = append(emphasized, SkillEntry{Name: s, Emphasis: true})
		}
	}
	allSkills := append(emphasized, regular...)

	// Build experience entries.
	experience := make([]ExperienceEntry, 0, len(profile.Experience))
	for _, exp := range profile.Experience {
		experience = append(experience, ExperienceEntry{
			Role:         exp.Role,
			Company:      exp.Company,
			Period:       exp.Period,
			Achievements: exp.Achievements,
		})
	}

	// Build project entries, prioritising variant-emphasised projects.
	projectEmphasisSet := make(map[string]bool, len(variant.ProjectEmphasis))
	for _, p := range variant.ProjectEmphasis {
		projectEmphasisSet[strings.ToLower(p)] = true
	}

	var emphProjects []ProjectEntry
	var otherProjects []ProjectEntry
	for _, proj := range profile.Projects {
		desc := proj.ShortDescription
		if desc == "" {
			desc = proj.Description
		}
		entry := ProjectEntry{
			Title:       proj.Title,
			Description: desc,
		}
		if projectEmphasisSet[strings.ToLower(proj.Title)] {
			emphProjects = append(emphProjects, entry)
		} else {
			otherProjects = append(otherProjects, entry)
		}
	}
	allProjects := append(emphProjects, otherProjects...)

	data := ResumeData{
		Name:       name,
		Headline:   variant.Headline,
		Email:      email,
		Website:    website,
		Location:   location,
		Summary:    variant.Summary,
		Experience: experience,
		Skills:     allSkills,
		Projects:   allProjects,
	}

	tmpl, err := template.New("resume.html").ParseFS(templates.FS, "resume.html")
	if err != nil {
		return "", fmt.Errorf("parse resume template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute resume template: %w", err)
	}

	return buf.String(), nil
}

// BuildCoverHTML renders the cover letter HTML template using profile data
// and the variant's cover template with company-specific placeholders filled in.
func BuildCoverHTML(variant Variant, profile *supabase.ProfileData, company, role, specificThing string) (string, error) {
	name, email, website, _ := profileConfig(profile)

	// Replace placeholders in the cover template.
	coverText := variant.CoverTemplate
	coverText = strings.ReplaceAll(coverText, "[COMPANY]", company)
	coverText = strings.ReplaceAll(coverText, "[ROLE]", role)
	coverText = strings.ReplaceAll(coverText, "[SPECIFIC THING]", specificThing)

	data := CoverData{
		Date:      "", // Will be set by caller or left for template default.
		Company:   company,
		CoverText: coverText,
		Name:      name,
		Email:     email,
		Website:   website,
	}

	tmpl, err := template.New("cover.html").ParseFS(templates.FS, "cover.html")
	if err != nil {
		return "", fmt.Errorf("parse cover template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute cover template: %w", err)
	}

	return buf.String(), nil
}
