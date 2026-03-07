# Migrate — Soul Product

Automated database and system migration tooling

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
| `analyze` | Analyze target for migrate insights |
| `report` | Generate migrate report |

## Development

```bash
# Build
go build -o migrate-go .

# Run (standalone)
./migrate-go --socket /tmp/migrate.sock
```

## Roadmap

- [ ] Implement core analysis engine
- [ ] Add domain-specific rules/checks
- [ ] Add fix/remediation capabilities
- [ ] Add streaming support
- [ ] Add report formatters (terminal, JSON, HTML)
- [ ] Write integration tests
