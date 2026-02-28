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

// BuildResumeHTML renders the resume HTML template using profile data
// tailored to the given variant.
func BuildResumeHTML(variant Variant, profile *supabase.ProfileData) (string, error) {
	// Extract site_config into a map for easy lookup.
	cfg := make(map[string]string, len(profile.SiteConfig))
	for _, row := range profile.SiteConfig {
		cfg[row.Key] = row.Value
	}

	// Build skills list with emphasis skills first.
	emphasisSet := make(map[string]bool, len(variant.SkillEmphasis))
	for _, s := range variant.SkillEmphasis {
		emphasisSet[strings.ToLower(s)] = true
	}

	var emphasized []SkillEntry
	var regular []SkillEntry
	for _, sk := range profile.Skills {
		entry := SkillEntry{Name: sk.Name}
		if emphasisSet[strings.ToLower(sk.Name)] || emphasisSet[strings.ToLower(sk.Category)] {
			entry.Emphasis = true
			emphasized = append(emphasized, entry)
		} else {
			regular = append(regular, entry)
		}
	}
	// Also add variant emphasis skills that may not be in the profile.
	existingSkills := make(map[string]bool)
	for _, sk := range profile.Skills {
		existingSkills[strings.ToLower(sk.Name)] = true
	}
	for _, s := range variant.SkillEmphasis {
		if !existingSkills[strings.ToLower(s)] {
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
		entry := ProjectEntry{
			Title:       proj.Title,
			Description: proj.Description,
		}
		if projectEmphasisSet[strings.ToLower(proj.Title)] {
			emphProjects = append(emphProjects, entry)
		} else {
			otherProjects = append(otherProjects, entry)
		}
	}
	allProjects := append(emphProjects, otherProjects...)

	data := ResumeData{
		Name:       cfg["name"],
		Headline:   variant.Headline,
		Email:      cfg["email"],
		Website:    cfg["website"],
		Location:   cfg["location"],
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
	cfg := make(map[string]string, len(profile.SiteConfig))
	for _, row := range profile.SiteConfig {
		cfg[row.Key] = row.Value
	}

	// Replace placeholders in the cover template.
	coverText := variant.CoverTemplate
	coverText = strings.ReplaceAll(coverText, "[COMPANY]", company)
	coverText = strings.ReplaceAll(coverText, "[ROLE]", role)
	coverText = strings.ReplaceAll(coverText, "[SPECIFIC THING]", specificThing)

	data := CoverData{
		Date:      "", // Will be set by caller or left for template default.
		Company:   company,
		CoverText: coverText,
		Name:      cfg["name"],
		Email:     cfg["email"],
		Website:   cfg["website"],
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
