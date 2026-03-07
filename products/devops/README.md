# DevOps — Soul Product

CI/CD pipeline and infrastructure automation

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
| `analyze` | Analyze target for devops insights |
| `report` | Generate devops report |

## Development

```bash
# Build
go build -o devops-go .

# Run (standalone)
./devops-go --socket /tmp/devops.sock
```

## Roadmap

- [ ] Implement core analysis engine
- [ ] Add domain-specific rules/checks
- [ ] Add fix/remediation capabilities
- [ ] Add streaming support
- [ ] Add report formatters (terminal, JSON, HTML)
- [ ] Write integration tests
