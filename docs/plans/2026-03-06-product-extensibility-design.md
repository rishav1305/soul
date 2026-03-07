# Product Extensibility — Zero-Code Product Registration

**Goal:** Adding a new product to Soul requires ONLY a binary + a config entry. Zero code changes for basic functionality (tools + task board). Dedicated frontend panels remain opt-in via a registry map.

**Tech Stack:** Go 1.24 backend, React 19 + TypeScript + Tailwind v4 frontend, gRPC product interface

---

## Current Problems

| Location | Issue |
|----------|-------|
| `cmd/soul/main.go:86-133` | Explicit startup code per product (flags, env vars, binary paths) |
| `web/src/components/layout/AppShell.tsx:34` | Hardcoded product set `['compliance', 'compliance-go', 'scout']` |
| `web/src/components/layout/ProductView.tsx:73-103` | if/else chain for dedicated panels |
| `web/src/components/layout/ProductRail.tsx:5-9` | Hardcoded color map |

---

## Design

### 1. Config File: `~/.soul/products.yaml`

```yaml
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

**Rules:**
- If file doesn't exist, Soul starts with no products (graceful degradation)
- Env vars `SOUL_<NAME>_BIN` override config file entries (backwards compat)
- `label` and `color` are optional — defaults derived from name
- `color` maps to existing stage color tokens: `active`, `brainstorm`, `validation`, `done`, `blocked`, `backlog`

### 2. Backend: Generic Product Startup (`main.go`)

Replace per-product startup blocks with a single loop:

```go
productConfigs := config.LoadProducts(cfg.DataDir)
for _, pc := range productConfigs {
    // Check env var override: SOUL_<UPPER_NAME>_BIN
    envKey := "SOUL_" + strings.ToUpper(strings.ReplaceAll(pc.Name, "-", "_")) + "_BIN"
    if envBin := os.Getenv(envKey); envBin != "" {
        pc.Binary = envBin
    }
    // Also check CLI flag --<name>-bin for backwards compat
    if flagBin := getFlagValue(args, "--"+pc.Name+"-bin"); flagBin != "" {
        pc.Binary = flagBin
    }
    if pc.Binary == "" { continue }
    if _, err := os.Stat(pc.Binary); err != nil { continue }
    manager.StartProduct(ctx, pc.Name, pc.Binary)
}
```

### 3. Config Parser (`internal/config/products.go`)

New file. Reads `~/.soul/products.yaml`:

```go
type ProductConfig struct {
    Name   string `yaml:"name"`
    Binary string `yaml:"binary"`
    Label  string `yaml:"label"`
    Color  string `yaml:"color"`
}

func LoadProducts(dataDir string) []ProductConfig
```

Uses `gopkg.in/yaml.v3` (already a common Go dependency, or use a simple custom parser to avoid new deps).

**Decision:** Use a minimal hand-rolled YAML parser for just this structure (flat list of objects with string fields) to avoid adding a dependency. The format is simple enough.

### 4. Backend: `GET /api/products` Endpoint

New route in `routes.go`. Returns registered product metadata:

```json
[
  {
    "name": "compliance",
    "version": "1.0.0",
    "label": "Compliance",
    "color": "validation",
    "tools": 5,
    "running": true
  },
  {
    "name": "scout",
    "version": "0.1.0",
    "label": "Career Intelligence",
    "color": "brainstorm",
    "tools": 6,
    "running": true
  }
]
```

Sources: product config (label, color) + registry manifest (version, tool count) + manager (running status).

### 5. Frontend: Fetch Products from API (`AppShell.tsx`)

Replace:
```tsx
const set = new Set<string>(['compliance', 'compliance-go', 'scout']);
```

With:
```tsx
const [apiProducts, setApiProducts] = useState<ProductInfo[]>([]);
useEffect(() => {
  fetch('/api/products').then(r => r.json()).then(setApiProducts);
}, []);

const products = useMemo(() => {
  const set = new Set<string>();
  for (const p of apiProducts) set.add(p.name);
  for (const t of planner.tasks) {
    if (t.product && t.product.toLowerCase() !== 'soul') set.add(t.product);
  }
  return Array.from(set).sort();
}, [apiProducts, planner.tasks]);
```

### 6. Frontend: Panel Registry (`ProductView.tsx`)

Replace if/else chain with a map:

```tsx
const DEDICATED_PANELS: Record<string, React.ComponentType> = {
  compliance: CompliancePanel,
  'compliance-go': CompliancePanel,
  scout: ScoutPanel,
};

// In render:
const Panel = DEDICATED_PANELS[activeProduct ?? ''];
if (Panel) {
  return (
    <div className="h-full flex flex-col">
      <ProductHeader name={activeProduct} metadata={productMetadata} />
      <div className="flex-1 overflow-hidden"><Panel /></div>
    </div>
  );
}
// Falls through to ProductTaskDashboard for unknown products
```

### 7. Frontend: Dynamic Colors/Labels (`ProductRail.tsx`)

Product metadata (label, color) comes from `/api/products` response, passed down as props. The hardcoded `PRODUCT_COLORS` map becomes a fallback for task-only products (not registered via API).

---

## Data Flow

```
STARTUP
  config.LoadProducts("~/.soul")
    → reads products.yaml
    → applies env var overrides (SOUL_<NAME>_BIN)
    → applies CLI flag overrides (--<name>-bin)
  for each product config:
    → manager.StartProduct(name, binary)
      → launches binary --socket <path>
      → gRPC GetManifest() → registry.Register()
  server stores product configs for /api/products

FRONTEND MOUNT
  fetch('/api/products')
    → [{name, version, label, color, tools, running}]
  merge with task-discovered products
  ProductRail renders all products
  ProductView checks DEDICATED_PANELS map → dedicated or generic dashboard

NEW PRODUCT ADDITION
  1. Build binary implementing ProductService gRPC
  2. Add entry to ~/.soul/products.yaml
  3. Restart Soul
  4. Product appears in rail, tools available to agent, task board works
  5. (Optional) Add dedicated panel component + one line in DEDICATED_PANELS map
```

---

## Files to Create/Modify

| File | Action | Purpose |
|------|--------|---------|
| `internal/config/products.go` | CREATE | ProductConfig struct, LoadProducts parser |
| `cmd/soul/main.go` | MODIFY | Replace per-product startup with config loop |
| `internal/server/routes.go` | MODIFY | Add `GET /api/products` route |
| `internal/server/handlers.go` | MODIFY | Add handleProductsList handler |
| `internal/server/server.go` | MODIFY | Store product configs on server struct |
| `web/src/lib/types.ts` | MODIFY | Add ProductInfo type |
| `web/src/components/layout/AppShell.tsx` | MODIFY | Fetch from /api/products, remove hardcoded set |
| `web/src/components/layout/ProductView.tsx` | MODIFY | Panel registry map instead of if/else |
| `web/src/components/layout/ProductRail.tsx` | MODIFY | Use API metadata for colors/labels |

---

## Backwards Compatibility

- `SOUL_COMPLIANCE_BIN` and `SOUL_SCOUT_BIN` env vars still work (override config)
- `--compliance-bin` and `--scout-bin` CLI flags still work (override config)
- If no `products.yaml` exists, falls back to env vars / CLI flags only
- Frontend gracefully handles empty product list
