# API — Soul Product

API generation, testing, and management automation

## Status

🚧 **Scaffolded** — Core structure in place, implementation pending.

## Architecture

This product follows the Soul product pattern:
- gRPC service implementing `ProductServiceServer`
- Communicates with Soul core via Unix socket
- Tools exposed via `GetManifest` and `ExecuteTool`

## Tools

| Tool | Description |
|------|-------------|
| `analyze` | Analyze target for api insights |
| `report` | Generate api report |

## Development

```bash
# Build
go build -o api-go .

# Run (standalone)
./api-go --socket /tmp/api.sock
```

## Roadmap

- [ ] Implement core analysis engine
- [ ] Add domain-specific rules/checks
- [ ] Add fix/remediation capabilities
- [ ] Add streaming support
- [ ] Add report formatters (terminal, JSON, HTML)
- [ ] Write integration tests
