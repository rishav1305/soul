package fix

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/rishav1305/soul/products/compliance/analyzers"
)

// unifiedDiff builds a minimal unified diff string showing the change at a
// specific line in the given file path.
func unifiedDiff(filePath string, lineNum int, oldLine string, newLine string) string {
	return fmt.Sprintf("--- a/%s\n+++ b/%s\n@@ -%d,1 +%d,1 @@\n-%s\n+%s\n",
		filePath, filePath, lineNum, lineNum, oldLine, newLine)
}

// readFileLines reads a file and returns its content split into lines.
func readFileLines(filePath string) ([]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", filePath, err)
	}
	return strings.Split(string(data), "\n"), nil
}

// joinLines joins lines back into a single string.
func joinLines(lines []string) string {
	return strings.Join(lines, "\n")
}

// secretToEnvStrategy replaces a hardcoded secret value on the finding line
// with a reference to an environment variable. For .ts/.js files it uses
// process.env.VAR_NAME, for .go files it uses os.Getenv("VAR_NAME"), and
// for .py files it uses os.environ["VAR_NAME"].
func secretToEnvStrategy(f analyzers.Finding) (string, string, error) {
	lines, err := readFileLines(f.File)
	if err != nil {
		return "", "", err
	}

	if f.Line < 1 || f.Line > len(lines) {
		return "", "", fmt.Errorf("line %d out of range (file has %d lines)", f.Line, len(lines))
	}

	oldLine := lines[f.Line-1]

	// Derive an environment variable name from the line content.
	envVarName := deriveEnvVarName(oldLine)

	// Determine the replacement based on file extension.
	var replacement string
	if strings.HasSuffix(f.File, ".go") {
		replacement = fmt.Sprintf(`os.Getenv("%s")`, envVarName)
	} else if strings.HasSuffix(f.File, ".py") {
		replacement = fmt.Sprintf(`os.environ["%s"]`, envVarName)
	} else {
		// Default to JS/TS style
		replacement = fmt.Sprintf("process.env.%s", envVarName)
	}

	// Replace the quoted secret value on the line with the env var reference.
	newLine := replaceQuotedSecret(oldLine, replacement)

	if newLine == oldLine {
		// Fallback: if we couldn't find a quoted string, just note it
		return "", "", fmt.Errorf("could not identify secret value to replace on line %d", f.Line)
	}

	lines[f.Line-1] = newLine
	newContent := joinLines(lines)
	patch := unifiedDiff(f.File, f.Line, oldLine, newLine)

	return patch, newContent, nil
}

// quotedStringRegex matches single or double quoted strings.
var quotedStringRegex = regexp.MustCompile(`(['"])[^'"]{4,}(['"])`)

// replaceQuotedSecret finds a quoted string on the line that looks like a
// secret and replaces it with the given replacement value.
func replaceQuotedSecret(line string, replacement string) string {
	return quotedStringRegex.ReplaceAllStringFunc(line, func(match string) string {
		// Keep variable names and short identifiers
		inner := match[1 : len(match)-1]
		if len(inner) < 4 {
			return match
		}
		// Replace with the env var reference (no quotes needed)
		return replacement
	})
}

// deriveEnvVarName tries to extract a meaningful variable name from the
// assignment line. For example, `const awsKey = "..."` -> `AWS_KEY`.
func deriveEnvVarName(line string) string {
	// Try to extract variable name from common assignment patterns
	patterns := []string{
		`(?:const|let|var|export)\s+(\w+)`,     // JS/TS: const varName = ...
		`(\w+)\s*[:=]\s*`,                       // Generic: varName = ... or key: ...
	}

	for _, p := range patterns {
		re := regexp.MustCompile(p)
		m := re.FindStringSubmatch(line)
		if len(m) > 1 {
			return toScreamingSnake(m[1])
		}
	}

	return "SECRET_VALUE"
}

// toScreamingSnake converts a camelCase or PascalCase identifier to
// SCREAMING_SNAKE_CASE.
func toScreamingSnake(s string) string {
	var result strings.Builder
	for i, ch := range s {
		if ch >= 'A' && ch <= 'Z' {
			if i > 0 {
				result.WriteByte('_')
			}
			result.WriteRune(ch)
		} else if ch >= 'a' && ch <= 'z' {
			result.WriteRune(ch - 'a' + 'A')
		} else {
			result.WriteRune(ch)
		}
	}
	return result.String()
}

// weakHashRegex matches createHash calls with md5 or sha1.
var weakHashRegex = regexp.MustCompile(`createHash\s*\(\s*['"](?:md5|sha1)['"]\s*\)`)

// weakHashStrategy replaces md5 or sha1 with sha256 in createHash calls.
func weakHashStrategy(f analyzers.Finding) (string, string, error) {
	lines, err := readFileLines(f.File)
	if err != nil {
		return "", "", err
	}

	if f.Line < 1 || f.Line > len(lines) {
		return "", "", fmt.Errorf("line %d out of range (file has %d lines)", f.Line, len(lines))
	}

	oldLine := lines[f.Line-1]
	newLine := weakHashRegex.ReplaceAllStringFunc(oldLine, func(match string) string {
		// Replace md5 or sha1 with sha256
		result := strings.Replace(match, "'md5'", "'sha256'", 1)
		result = strings.Replace(result, "\"md5\"", "\"sha256\"", 1)
		result = strings.Replace(result, "'sha1'", "'sha256'", 1)
		result = strings.Replace(result, "\"sha1\"", "\"sha256\"", 1)
		return result
	})

	if newLine == oldLine {
		return "", "", fmt.Errorf("could not find weak hash to replace on line %d", f.Line)
	}

	lines[f.Line-1] = newLine
	newContent := joinLines(lines)
	patch := unifiedDiff(f.File, f.Line, oldLine, newLine)

	return patch, newContent, nil
}

// dangerousCallRegex matches lines containing dangerous dynamic code execution calls.
var dangerousCallRegex = regexp.MustCompile(`\beval\s*\(`)

// evalRemovalStrategy comments out the dangerous call and adds a TODO comment.
// This strategy detects and remediates dangerous dynamic code execution patterns
// found in scanned codebases -- the tool itself never executes arbitrary code.
func evalRemovalStrategy(f analyzers.Finding) (string, string, error) {
	lines, err := readFileLines(f.File)
	if err != nil {
		return "", "", err
	}

	if f.Line < 1 || f.Line > len(lines) {
		return "", "", fmt.Errorf("line %d out of range (file has %d lines)", f.Line, len(lines))
	}

	oldLine := lines[f.Line-1]

	if !dangerousCallRegex.MatchString(oldLine) {
		return "", "", fmt.Errorf("could not find dangerous call on line %d", f.Line)
	}

	// Determine comment prefix based on file extension.
	commentPrefix := "//"
	if strings.HasSuffix(f.File, ".py") || strings.HasSuffix(f.File, ".rb") {
		commentPrefix = "#"
	}

	// Preserve leading whitespace.
	trimmed := strings.TrimLeft(oldLine, " \t")
	indent := oldLine[:len(oldLine)-len(trimmed)]

	newLine := fmt.Sprintf("%s%s TODO: Replace eval() with a safe alternative\n%s%s %s",
		indent, commentPrefix, indent, commentPrefix, trimmed)

	lines[f.Line-1] = newLine
	newContent := joinLines(lines)
	patch := unifiedDiff(f.File, f.Line, oldLine, newLine)

	return patch, newContent, nil
}

// corsWildcardFixRegex matches CORS wildcard origin patterns.
var corsWildcardFixRegex = regexp.MustCompile(`(['"])\*(['"])`)

// corsStrategy replaces CORS wildcard '*' with a specific origin placeholder.
func corsStrategy(f analyzers.Finding) (string, string, error) {
	lines, err := readFileLines(f.File)
	if err != nil {
		return "", "", err
	}

	if f.Line < 1 || f.Line > len(lines) {
		return "", "", fmt.Errorf("line %d out of range (file has %d lines)", f.Line, len(lines))
	}

	oldLine := lines[f.Line-1]

	// Replace '*' with a placeholder origin URL.
	newLine := corsWildcardFixRegex.ReplaceAllString(oldLine, "${1}https://your-domain.com${2}")

	if newLine == oldLine {
		return "", "", fmt.Errorf("could not find CORS wildcard to replace on line %d", f.Line)
	}

	lines[f.Line-1] = newLine
	newContent := joinLines(lines)
	patch := unifiedDiff(f.File, f.Line, oldLine, newLine)

	return patch, newContent, nil
}
