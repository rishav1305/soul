package scan

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rishav1305/soul-v2/internal/quality/compliance/analyzers"
	"github.com/rishav1305/soul-v2/internal/quality/compliance/rules"
)

// skipDirs are directories excluded from scanning.
var skipDirs = map[string]bool{
	".git":        true,
	"node_modules": true,
	"dist":        true,
	"build":       true,
	"vendor":      true,
	".next":       true,
	"__pycache__": true,
}

// maxScanFileSize is the maximum file size to scan (1MB).
const maxScanFileSize int64 = 1 * 1024 * 1024

// ScanDirectory performs a compliance scan on the given directory.
func ScanDirectory(opts ScanOptions) (*ScanResult, error) {
	start := time.Now()

	if opts.Directory == "" {
		return nil, fmt.Errorf("directory is required")
	}

	// Build exclude set from opts.Exclude (normalize to forward slashes).
	excludeSet := make(map[string]bool, len(opts.Exclude))
	for _, p := range opts.Exclude {
		excludeSet[filepath.ToSlash(p)] = true
	}

	// Walk directory and collect files.
	var files []analyzers.ScannedFile
	absDir, err := filepath.Abs(opts.Directory)
	if err != nil {
		return nil, fmt.Errorf("resolving directory: %w", err)
	}

	err = filepath.WalkDir(absDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, relErr := filepath.Rel(absDir, path)
		if relErr != nil {
			return nil
		}
		relSlash := filepath.ToSlash(relPath)

		// Check exclude list.
		for exc := range excludeSet {
			if strings.HasPrefix(relSlash, exc) || relSlash == exc {
				return nil
			}
		}

		info, infoErr := d.Info()
		if infoErr != nil {
			return nil
		}

		if info.Size() > maxScanFileSize {
			return nil
		}

		ext := strings.TrimPrefix(filepath.Ext(path), ".")
		files = append(files, analyzers.ScannedFile{
			Path:         path,
			RelativePath: relPath,
			Extension:    ext,
			Size:         info.Size(),
		})

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking directory: %w", err)
	}

	// Load rules.
	allRules, err := rules.LoadAll(opts.Frameworks)
	if err != nil {
		return nil, fmt.Errorf("loading rules: %w", err)
	}

	// Create analyzers.
	allAnalyzers := []analyzers.Analyzer{
		&analyzers.SecretScanner{},
		&analyzers.ConfigChecker{},
		&analyzers.GitAnalyzer{},
		&analyzers.DepAuditor{},
		&analyzers.ASTAnalyzer{},
	}

	// Filter analyzers if opts.Analyzers is set.
	if len(opts.Analyzers) > 0 {
		want := make(map[string]bool, len(opts.Analyzers))
		for _, name := range opts.Analyzers {
			want[strings.ToLower(name)] = true
		}
		var filtered []analyzers.Analyzer
		for _, a := range allAnalyzers {
			if want[strings.ToLower(a.Name())] {
				filtered = append(filtered, a)
			}
		}
		allAnalyzers = filtered
	}

	// Run analyzers in parallel.
	var mu sync.Mutex
	var wg sync.WaitGroup
	var allFindings []analyzers.Finding
	var analyzerNames []string

	for _, a := range allAnalyzers {
		analyzerNames = append(analyzerNames, a.Name())
	}

	wg.Add(len(allAnalyzers))
	for _, a := range allAnalyzers {
		go func(az analyzers.Analyzer) {
			defer wg.Done()
			results, analyzeErr := az.Analyze(files, allRules)
			if analyzeErr != nil {
				return
			}
			mu.Lock()
			allFindings = append(allFindings, results...)
			mu.Unlock()
		}(a)
	}
	wg.Wait()

	// Deduplicate on file:line:id key.
	seen := make(map[string]bool)
	var deduped []analyzers.Finding
	for _, f := range allFindings {
		key := fmt.Sprintf("%s:%d:%s", f.File, f.Line, f.ID)
		if seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, f)
	}

	// Filter by severity if requested.
	if len(opts.Severity) > 0 {
		wantSev := make(map[string]bool, len(opts.Severity))
		for _, s := range opts.Severity {
			wantSev[strings.ToLower(s)] = true
		}
		var filtered []analyzers.Finding
		for _, f := range deduped {
			if wantSev[strings.ToLower(f.Severity)] {
				filtered = append(filtered, f)
			}
		}
		deduped = filtered
	}

	// Ensure non-nil slice for JSON serialization.
	if deduped == nil {
		deduped = []analyzers.Finding{}
	}

	// Build summary.
	summary := ScanSummary{
		Total:       len(deduped),
		BySeverity:  make(map[string]int),
		ByFramework: make(map[string]int),
		ByAnalyzer:  make(map[string]int),
	}
	for _, f := range deduped {
		summary.BySeverity[f.Severity]++
		summary.ByAnalyzer[f.Analyzer]++
		if f.Fixable {
			summary.Fixable++
		}
		for _, fw := range f.Framework {
			summary.ByFramework[fw]++
		}
	}

	duration := time.Since(start).Seconds()

	return &ScanResult{
		Findings: deduped,
		Summary:  summary,
		Metadata: ScanMetadata{
			Directory:    absDir,
			Duration:     duration,
			AnalyzersRun: analyzerNames,
			Timestamp:    start.UTC().Format(time.RFC3339),
		},
	}, nil
}
