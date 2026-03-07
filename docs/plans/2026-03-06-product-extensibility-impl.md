# Product Extensibility Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove all hardcoded product names from Soul so adding a new product requires only a binary + a config entry in `~/.soul/products.yaml`.

**Architecture:** Config-driven product discovery replaces per-product startup code in main.go. New `/api/products` endpoint feeds the frontend. Frontend replaces hardcoded product set and if/else panel chain with API-driven discovery and a panel registry map.

**Tech Stack:** Go 1.24, React 19 + TypeScript + Tailwind v4, gRPC product interface

---

## Task 1: Product Config Parser

**Files:**
- Create: `internal/config/products.go`

**Step 1: Create the ProductConfig type and LoadProducts function**

Create `internal/config/products.go`:

```go
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
```

**Step 2: Verify build**

Run: `go build ./...`
Expected: Success

---

## Task 2: Create Default products.yaml

**Files:**
- Create: `~/.soul/products.yaml`

**Step 1: Write the config file with current products**

Create `~/.soul/products.yaml`:

```yaml
# Soul product configuration
# Each product needs: name, binary. Label and color are optional.

products:
  - name: compliance
    binary: /home/rishav/soul/products/compliance-go/compliance-go
    label: Compliance
    color: validation
  - name: scout
    binary: /home/rishav/soul/products/scout/scout
    label: Career Intelligence
    color: brainstorm
```

---

## Task 3: Generic Product Startup in main.go

**Files:**
- Modify: `cmd/soul/main.go:44-156`

**Step 1: Replace per-product startup blocks with config-driven loop**

Replace lines 86-133 (the compliance and scout startup blocks) with:

```go
	// Start products from config file + env var / CLI flag overrides.
	productConfigs := config.LoadProducts(cfg.DataDir)

	// Backwards compat: if no config file, check legacy env vars / flags.
	if len(productConfigs) == 0 {
		// Legacy compliance
		compBin := getFlagValue(args, "--compliance-bin")
		if compBin == "" {
			compBin = os.Getenv("SOUL_COMPLIANCE_BIN")
		}
		if compBin != "" {
			productConfigs = append(productConfigs, config.ProductConfig{
				Name: "compliance", Binary: compBin, Label: "Compliance", Color: "validation",
			})
		}
		// Legacy scout
		scoutBin := getFlagValue(args, "--scout-bin")
		if scoutBin == "" {
			scoutBin = os.Getenv("SOUL_SCOUT_BIN")
		}
		if scoutBin == "" {
			cwd, _ := os.Getwd()
			candidate := filepath.Join(cwd, "products", "scout", "scout")
			if _, err := os.Stat(candidate); err == nil {
				scoutBin = candidate
			}
		}
		if scoutBin != "" {
			productConfigs = append(productConfigs, config.ProductConfig{
				Name: "scout", Binary: scoutBin, Label: "Career Intelligence", Color: "brainstorm",
			})
		}
	} else {
		// Config file exists — apply env var / CLI flag overrides per product.
		for i := range productConfigs {
			pc := &productConfigs[i]
			envKey := "SOUL_" + strings.ToUpper(strings.ReplaceAll(pc.Name, "-", "_")) + "_BIN"
			if envBin := os.Getenv(envKey); envBin != "" {
				pc.Binary = envBin
			}
			if flagBin := getFlagValue(args, "--"+pc.Name+"-bin"); flagBin != "" {
				pc.Binary = flagBin
			}
		}
	}

	ctx := context.Background()
	for _, pc := range productConfigs {
		if pc.Binary == "" {
			continue
		}
		if _, err := os.Stat(pc.Binary); err != nil {
			log.Printf("WARNING: %s binary not found at %s", pc.Name, pc.Binary)
			continue
		}
		fmt.Printf("  Starting %s product: %s\n", pc.Name, pc.Binary)
		if err := manager.StartProduct(ctx, pc.Name, pc.Binary); err != nil {
			log.Printf("WARNING: failed to start %s product: %v", pc.Name, err)
		} else {
			fmt.Printf("  %s product started\n", pc.Name)
		}
	}
```

Also add `"strings"` to the import block at the top of main.go (it's not currently imported).

**Step 2: Pass productConfigs to the server**

After the product startup loop and before `srv := server.NewWithWebFS(...)`, we need to pass product configs to the server. Modify the `NewWithWebFS` call:

Change line 136:
```go
srv := server.NewWithWebFS(cfg, manager, aiClient, plannerStore, soul.WebDist)
```
To:
```go
srv := server.NewWithWebFS(cfg, manager, aiClient, plannerStore, soul.WebDist, productConfigs)
```

**Step 3: Update printHelp to be generic**

Replace the help text (lines 158-176) to mention generic product config instead of per-product flags:

```go
func printHelp() {
	fmt.Printf("◆ Soul v%s\n\n", version)
	fmt.Println("Usage:")
	fmt.Println("  soul serve [--port PORT] [--dev]   Start web UI")
	fmt.Println("  soul --version                     Show version")
	fmt.Println("  soul --help                        Show this help")
	fmt.Println()
	fmt.Println("Authentication (in priority order):")
	fmt.Println("  ANTHROPIC_API_KEY      Claude API key")
	fmt.Println("  ~/.claude/.credentials.json   Claude Max/Pro OAuth (auto-detected)")
	fmt.Println()
	fmt.Println("Products:")
	fmt.Println("  Configure in ~/.soul/products.yaml")
	fmt.Println("  Override binary: SOUL_<NAME>_BIN or --<name>-bin flag")
	fmt.Println()
	fmt.Println("Environment:")
	fmt.Println("  SOUL_PORT              Server port (default: 3000)")
	fmt.Println("  SOUL_HOST              Server host (default: 127.0.0.1)")
	fmt.Println("  SOUL_DATA_DIR          Data directory (default: ~/.soul)")
	fmt.Println("  SOUL_MODEL             Claude model (default: claude-sonnet-4-6)")
}
```

**Step 4: Verify build**

Run: `go build ./...`
Expected: Will fail because NewWithWebFS signature changed — that's expected, we fix it in next task.

---

## Task 4: Store Product Configs on Server + Registry Names Method

**Files:**
- Modify: `internal/server/server.go:31-51` (Server struct)
- Modify: `internal/server/server.go:55-98` (New constructor)
- Modify: `internal/server/server.go:100-156` (NewWithWebFS constructor)
- Modify: `internal/products/registry.go` (add Names method)

**Step 1: Add productConfigs field to Server struct**

In `server.go`, add to Server struct (after line 46, the `skillStore` field):

```go
	productConfigs []config.ProductConfig // from products.yaml
```

**Step 2: Update NewWithWebFS to accept productConfigs**

Change the `NewWithWebFS` signature at line 101:

```go
func NewWithWebFS(cfg config.Config, pm *products.Manager, aiClient *ai.Client, plannerStore *planner.Store, webDist embed.FS, productConfigs []config.ProductConfig) *Server {
```

In the struct initialization (around line 120-131), add:

```go
		productConfigs: productConfigs,
```

**Step 3: Update New() to also accept productConfigs (or pass nil)**

Change the `New` signature at line 55:

```go
func New(cfg config.Config, pm *products.Manager, aiClient *ai.Client, plannerStore *planner.Store) *Server {
```

This one can stay as-is (without productConfigs) since it's only used if NewWithWebFS doesn't exist. The productConfigs field will be nil, which is fine.

**Step 4: Add Names() method to Registry**

In `internal/products/registry.go`, add after the `AllTools` method:

```go
// Names returns all registered product names.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.manifests))
	for name := range r.manifests {
		names = append(names, name)
	}
	return names
}

// GetManifest returns the manifest for a product (exported alias for Get).
func (r *Registry) GetManifest(name string) (*soulv1.Manifest, bool) {
	return r.Get(name)
}
```

**Step 5: Verify build**

Run: `go build ./...`
Expected: Success

---

## Task 5: `/api/products` Endpoint

**Files:**
- Modify: `internal/server/routes.go:16-17` (add route)
- Modify: `internal/server/routes.go` (add handler, append after handleToolsList)

**Step 1: Add route**

In `routes.go`, after line 21 (`GET /api/tools`), add:

```go
	// Products list endpoint — returns registered product metadata.
	s.mux.HandleFunc("GET /api/products", s.handleProductsList)
```

**Step 2: Add handler**

After the `handleToolsList` function (after line 126), add:

```go
// productInfo is the JSON-serializable product metadata returned by /api/products.
type productInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Label   string `json:"label"`
	Color   string `json:"color"`
	Tools   int    `json:"tools"`
	Running bool   `json:"running"`
}

// handleProductsList returns all known products with metadata.
func (s *Server) handleProductsList(w http.ResponseWriter, r *http.Request) {
	// Build a map from product configs for label/color lookup.
	configMap := make(map[string]config.ProductConfig)
	for _, pc := range s.productConfigs {
		configMap[pc.Name] = pc
	}

	var result []productInfo

	if s.products != nil {
		for _, name := range s.products.Registry().Names() {
			pi := productInfo{
				Name:    name,
				Running: true,
			}

			// Enrich from manifest.
			if manifest, ok := s.products.Registry().GetManifest(name); ok {
				pi.Version = manifest.GetVersion()
				pi.Tools = len(manifest.GetTools())
			}

			// Enrich from config.
			if pc, ok := configMap[name]; ok {
				pi.Label = pc.Label
				pi.Color = pc.Color
			}

			// Default label from name if not set.
			if pi.Label == "" {
				pi.Label = strings.Title(strings.ReplaceAll(name, "-", " "))
			}

			result = append(result, pi)
		}
	}

	if result == nil {
		result = []productInfo{}
	}
	writeJSON(w, http.StatusOK, result)
}
```

Note: `strings.Title` is deprecated but works fine here. If the linter complains, use `cases.Title(language.English).String(...)` from `golang.org/x/text` or just capitalize manually.

**Step 3: Add config import**

In `routes.go`, add to imports:

```go
	"github.com/rishav1305/soul/internal/config"
```

**Step 4: Verify build**

Run: `go build ./...`
Expected: Success

**Step 5: Build binary and test endpoint**

Run: `go build -o soul ./cmd/soul`
Then: `curl -s http://localhost:3000/api/products | python3 -m json.tool`
Expected: JSON array with compliance and scout entries (after server restart).

---

## Task 6: Frontend — ProductInfo Type

**Files:**
- Modify: `web/src/lib/types.ts`

**Step 1: Add ProductInfo type**

Add at the end of the types file (before any closing braces):

```typescript
/** Product metadata from /api/products */
export interface ProductInfo {
  name: string;
  version: string;
  label: string;
  color: string;
  tools: number;
  running: boolean;
}
```

**Step 2: Verify build**

Run: `cd web && npx vite build`
Expected: Success

---

## Task 7: Frontend — Fetch Products from API (AppShell.tsx)

**Files:**
- Modify: `web/src/components/layout/AppShell.tsx:1-40`

**Step 1: Add useEffect import and ProductInfo import**

Change the import on line 1 from:
```tsx
import { useState, useMemo, useCallback } from 'react';
```
To:
```tsx
import { useState, useMemo, useCallback, useEffect } from 'react';
```

Add to the types import (line 9):
```tsx
import type { PlannerTask, TaskStage, TaskFilters, ProductInfo } from '../../lib/types.ts';
```

**Step 2: Replace hardcoded product set with API-driven discovery**

Replace lines 32-40:
```tsx
  // Derive unique products dynamically from tasks (must be before useProductContext)
  const products = useMemo(() => {
    const set = new Set<string>(['compliance', 'compliance-go', 'scout']);
    for (const t of planner.tasks) {
      // Exclude 'soul' — that's the platform itself, not a product
      if (t.product && t.product.toLowerCase() !== 'soul') set.add(t.product);
    }
    return Array.from(set).sort();
  }, [planner.tasks]);
```

With:
```tsx
  // Fetch registered products from API on mount
  const [apiProducts, setApiProducts] = useState<ProductInfo[]>([]);
  useEffect(() => {
    fetch('/api/products')
      .then((r) => r.json())
      .then((data: ProductInfo[]) => setApiProducts(data))
      .catch(() => {});
  }, []);

  // Merge API products with task-discovered products
  const products = useMemo(() => {
    const set = new Set<string>();
    for (const p of apiProducts) set.add(p.name);
    for (const t of planner.tasks) {
      if (t.product && t.product.toLowerCase() !== 'soul') set.add(t.product);
    }
    return Array.from(set).sort();
  }, [apiProducts, planner.tasks]);

  // Build product metadata map for downstream components
  const productMetadata = useMemo(() => {
    const map = new Map<string, ProductInfo>();
    for (const p of apiProducts) map.set(p.name, p);
    return map;
  }, [apiProducts]);
```

**Step 3: Pass productMetadata to ProductRail and ProductView**

This needs `productMetadata` threaded through — we'll add the prop in the next tasks.

**Step 4: Verify build**

Run: `cd web && npx vite build`
Expected: Success (may have unused variable warnings for `productMetadata` — that's fine for now)

---

## Task 8: Frontend — Panel Registry (ProductView.tsx)

**Files:**
- Modify: `web/src/components/layout/ProductView.tsx:1-103`

**Step 1: Add ProductInfo import and metadata prop**

Add to imports:
```tsx
import type { ProductInfo } from '../../lib/types.ts';
```

Add to `ProductViewProps` interface (after line 15):
```tsx
  productMetadata?: Map<string, ProductInfo>;
```

Add to function params (after `activeProduct`):
```tsx
  productMetadata,
```

**Step 2: Replace if/else chain with panel registry**

Replace lines 71-103 (the dedicated product dashboard section):

```tsx
  // ── Panel registry — dedicated panels for known products ──────────────────
  const DEDICATED_PANELS: Record<string, React.ComponentType> = {
    compliance: CompliancePanel,
    'compliance-go': CompliancePanel,
    scout: ScoutPanel,
  };

  if (activeProduct) {
    const Panel = DEDICATED_PANELS[activeProduct];
    if (Panel) {
      const meta = productMetadata?.get(activeProduct);
      return (
        <div className="h-full flex flex-col">
          <div className="glass flex items-center gap-2 px-4 h-11 shrink-0 border-b border-border-subtle">
            <span className="font-display text-sm font-semibold text-fg">{meta?.label || activeProduct}</span>
            {meta?.label && meta.label !== activeProduct && (
              <span className="text-[10px] px-2 py-0.5 rounded bg-soul/10 text-soul font-medium">
                {meta.label}
              </span>
            )}
          </div>
          <div className="flex-1 overflow-hidden">
            <Panel />
          </div>
        </div>
      );
    }

    // Any other product gets generic task dashboard
    return (
      <ProductTaskDashboard
        product={activeProduct}
```
(rest of ProductTaskDashboard props unchanged)

**Step 3: Verify build**

Run: `cd web && npx vite build`
Expected: Success

---

## Task 9: Frontend — Dynamic Colors in ProductRail

**Files:**
- Modify: `web/src/components/layout/ProductRail.tsx:1-9`
- Modify: `web/src/components/layout/ProductRail.tsx:294-348` (props interface + function)

**Step 1: Add ProductInfo import and metadata prop**

Add to imports (line 2):
```tsx
import type { PlannerTask, PanelPosition, DrawerLayout, ProductInfo } from '../../lib/types.ts';
```

Add to `ProductRailProps` interface (after `tasks` on line 298):
```tsx
  productMetadata?: Map<string, ProductInfo>;
```

Add to function destructuring:
```tsx
  productMetadata,
```

**Step 2: Enhance productColor to use API metadata**

Replace the `productColor` function (lines 20-25) with:

```tsx
function productColor(name: string, metadata?: Map<string, ProductInfo>): string {
  // Check hardcoded map first (keeps existing colors stable)
  if (PRODUCT_COLORS[name]) return PRODUCT_COLORS[name];

  // Check API metadata for color hint
  const meta = metadata?.get(name);
  if (meta?.color) {
    const colorMap: Record<string, string> = {
      active: 'text-stage-active bg-stage-active/10 border-stage-active',
      brainstorm: 'text-stage-brainstorm bg-stage-brainstorm/10 border-stage-brainstorm',
      validation: 'text-stage-validation bg-stage-validation/10 border-stage-validation',
      done: 'text-stage-done bg-stage-done/10 border-stage-done',
      blocked: 'text-stage-blocked bg-stage-blocked/10 border-stage-blocked',
      backlog: 'text-stage-backlog bg-stage-backlog/10 border-stage-backlog',
    };
    if (colorMap[meta.color]) return colorMap[meta.color];
  }

  // Fallback: hash-based color
  let hash = 0;
  for (let i = 0; i < name.length; i++) hash = (hash * 31 + name.charCodeAt(i)) & 0xffff;
  return FALLBACK_COLORS[hash % FALLBACK_COLORS.length];
}
```

**Step 3: Update productColor calls to pass metadata**

In the product list rendering (~line 411 and ~line 447), change:
```tsx
const colors = productColor(product);
```
To:
```tsx
const colors = productColor(product, productMetadata);
```

**Step 4: Verify build**

Run: `cd web && npx vite build`
Expected: Success

---

## Task 10: Wire productMetadata Through AppShell

**Files:**
- Modify: `web/src/components/layout/AppShell.tsx` (where ProductRail and ProductView are rendered)

**Step 1: Pass productMetadata to ProductRail**

Find where `<ProductRail` is rendered in AppShell.tsx and add:
```tsx
productMetadata={productMetadata}
```

**Step 2: Pass productMetadata to ProductView**

Find where `<ProductView` is rendered and add:
```tsx
productMetadata={productMetadata}
```

**Step 3: Verify build**

Run: `cd web && npx vite build`
Expected: Success

---

## Task 11: Build, Restart, Verify

**Step 1: Build Go binary**

Run: `cd /home/rishav/soul && go build -o soul ./cmd/soul`
Expected: Success

**Step 2: Build frontend**

Run: `cd web && npx vite build`
Expected: Success

**Step 3: Test /api/products endpoint**

Run: `curl -s http://localhost:3000/api/products | python3 -m json.tool`
Expected: JSON array with compliance and scout entries including label, color, version, tools count.

**Step 4: Verify frontend loads products from API**

Open browser to http://192.168.0.128:3000
Expected: Product rail shows compliance and scout (same as before — no regression)

**Step 5: Verify no hardcoded product names remain**

Check that removing a product from `products.yaml` and restarting removes it from the rail.

---

## Key Files Reference

| File | Lines | What |
|------|-------|------|
| `internal/config/products.go` | NEW | ProductConfig struct, LoadProducts YAML parser |
| `~/.soul/products.yaml` | NEW | Product config file |
| `cmd/soul/main.go` | 86-133 | Replace per-product startup with config loop |
| `internal/server/server.go` | 31-51 | Add productConfigs field |
| `internal/server/server.go` | 100-156 | Update NewWithWebFS signature |
| `internal/server/routes.go` | 16-17 | Add GET /api/products route |
| `internal/server/routes.go` | 126+ | handleProductsList handler |
| `internal/products/registry.go` | EOF | Add Names() method |
| `web/src/lib/types.ts` | EOF | Add ProductInfo type |
| `web/src/components/layout/AppShell.tsx` | 32-40 | API-driven product discovery |
| `web/src/components/layout/ProductView.tsx` | 71-103 | Panel registry map |
| `web/src/components/layout/ProductRail.tsx` | 20-25 | Dynamic colors from API |
