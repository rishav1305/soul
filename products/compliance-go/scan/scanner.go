package scan

import (
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/rishav1305/soul/products/compliance-go/analyzers"
)

// defaultExcludeDirs lists directory names that are always skipped during scanning.
var defaultExcludeDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"dist":         true,
	"build":        true,
	"vendor":       true,
	".next":        true,
	"__pycache__":  true,
}

// maxScanFileSize is the maximum file size (1 MB) that will be included in scan results.
const maxScanFileSize int64 = 1024 * 1024

// ScanDirectory walks the directory tree rooted at dir, collecting files
// while skipping default excluded directories, any directories in the exclude
// list, and files larger than 1 MB.
func ScanDirectory(dir string, exclude []string) ([]analyzers.ScannedFile, error) {
	// Merge custom excludes into a lookup set.
	skipDirs := make(map[string]bool, len(defaultExcludeDirs)+len(exclude))
	for k, v := range defaultExcludeDirs {
		skipDirs[k] = v
	}
	for _, e := range exclude {
		skipDirs[e] = true
	}

	var files []analyzers.ScannedFile

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip entries that can't be read
		}

		// Skip excluded directories.
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		// Get file info for size check.
		info, err := d.Info()
		if err != nil {
			return nil // skip files whose info can't be read
		}

		// Skip files larger than 1 MB.
		if info.Size() > maxScanFileSize {
			return nil
		}

		// Compute relative path from the root directory.
		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return nil
		}

		// Extract extension without the leading dot.
		ext := strings.TrimPrefix(filepath.Ext(path), ".")

		files = append(files, analyzers.ScannedFile{
			Path:         path,
			RelativePath: relPath,
			Extension:    ext,
			Size:         info.Size(),
		})

		return nil
	})

	return files, err
}
