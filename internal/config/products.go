package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// ProductConfig describes a product to start from products.yaml.
type ProductConfig struct {
	Name   string // product name (required)
	Binary string // path to binary (required)
	Label  string // display label (optional, defaults to Name)
	Color  string // stage color token (optional, defaults to "")
}

// LoadProducts reads ~/.soul/products.yaml and returns product configs.
// Returns nil (not error) if the file doesn't exist — that's normal.
func LoadProducts(dataDir string) []ProductConfig {
	path := filepath.Join(dataDir, "products.yaml")
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var products []ProductConfig
	var current *ProductConfig
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and comments
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Skip the top-level "products:" key
		if trimmed == "products:" {
			continue
		}

		// New list item: "- name: value"
		if strings.HasPrefix(trimmed, "- ") {
			if current != nil && current.Name != "" {
				products = append(products, *current)
			}
			current = &ProductConfig{}
			trimmed = strings.TrimPrefix(trimmed, "- ")
		}

		if current == nil {
			continue
		}

		// Parse "key: value"
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch key {
		case "name":
			current.Name = val
		case "binary":
			current.Binary = val
		case "label":
			current.Label = val
		case "color":
			current.Color = val
		}
	}

	// Don't forget the last entry
	if current != nil && current.Name != "" {
		products = append(products, *current)
	}

	return products
}
