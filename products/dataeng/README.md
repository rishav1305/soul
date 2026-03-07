# DataEng — Soul Product

Build and manage end-to-end data pipelines

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
| `analyze` | Analyze target for dataeng insights |
| `report` | Generate dataeng report |

## Development

```bash
# Build
go build -o dataeng-go .

# Run (standalone)
./dataeng-go --socket /tmp/dataeng.sock
```

## Roadmap

- [ ] Implement core analysis engine
- [ ] Add domain-specific rules/checks
- [ ] Add fix/remediation capabilities
- [ ] Add streaming support
- [ ] Add report formatters (terminal, JSON, HTML)
- [ ] Write integration tests
