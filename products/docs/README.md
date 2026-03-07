# Docs — Soul Product

Automated documentation generation and maintenance

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
| `analyze` | Analyze target for docs insights |
| `report` | Generate docs report |

## Development

```bash
# Build
go build -o docs-go .

# Run (standalone)
./docs-go --socket /tmp/docs.sock
```

## Roadmap

- [ ] Implement core analysis engine
- [ ] Add domain-specific rules/checks
- [ ] Add fix/remediation capabilities
- [ ] Add streaming support
- [ ] Add report formatters (terminal, JSON, HTML)
- [ ] Write integration tests
