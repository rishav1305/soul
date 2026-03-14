package executor

import "strings"

var microKeywords = []string{
	"add button", "add icon", "change color", "fix typo", "rename",
	"update text", "update label", "change text", "toggle", "hide",
	"show", "move button", "add tooltip", "remove button", "add link",
	"change icon", "fix spacing", "fix padding", "fix margin", "add class",
	"change style", "update style", "add prop",
}

var fullKeywords = []string{
	"refactor", "redesign", "new feature", "add api", "add endpoint",
	"database", "migration", "security", "authentication", "pipeline",
	"architect",
}

// ClassifyWorkflow returns "micro", "quick", or "full" based on task title and description.
func ClassifyWorkflow(title, description string) string {
	combined := strings.ToLower(title + " " + description)

	for _, kw := range fullKeywords {
		if strings.Contains(combined, kw) {
			return "full"
		}
	}

	for _, kw := range microKeywords {
		if strings.Contains(combined, kw) {
			return "micro"
		}
	}

	return "quick"
}

// IterationLimit returns the maximum number of iterations allowed for a given workflow type.
func IterationLimit(workflow string) int {
	switch workflow {
	case "micro":
		return 15
	case "quick":
		return 30
	default:
		return 40
	}
}
