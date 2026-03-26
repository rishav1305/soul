#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

echo "=== Soul v2 Deploy ==="

# 1. Build frontend
echo "Building frontend..."
cd "$PROJECT_DIR/web"
npm run build

# 2. Build ALL Go binaries (explicit -o to ensure binary is actually updated)
echo "Building Go binaries..."
cd "$PROJECT_DIR"
go build -o soul-chat     ./cmd/chat
go build -o soul-tasks    ./cmd/tasks
go build -o soul-tutor    ./cmd/tutor
go build -o soul-projects ./cmd/projects
go build -o soul-observe  ./cmd/observe
go build -o soul-mcp      ./cmd/mcp
go build -o soul-infra    ./cmd/infra
go build -o soul-quality  ./cmd/quality
go build -o soul-data     ./cmd/data
go build -o soul-docs     ./cmd/docs
go build -o soul-sentinel ./cmd/sentinel
go build -o soul-bench    ./cmd/bench
go build -o soul-mesh     ./cmd/mesh
go build -o soul-scout    ./cmd/scout
echo "All 14 binaries built."

# 3. Run tests
echo "Running tests..."
go test -race -count=1 ./...

# 4. Install systemd services and restart all active ones
echo "Installing systemd services..."
if grep -q 'CHANGE_ME' deploy/soul-v2.service; then
    echo "ERROR: SOUL_V2_AUTH_TOKEN is still CHANGE_ME in deploy/soul-v2.service"
    echo "Generate a token: openssl rand -hex 16"
    exit 1
fi

for svc in deploy/soul-v2*.service; do
    sudo cp "$svc" /etc/systemd/system/
done
sudo systemctl daemon-reload

# Restart all active soul-v2 services
for svc in soul-v2 soul-v2-tasks soul-v2-tutor soul-v2-scout; do
    if systemctl is-enabled "$svc" 2>/dev/null; then
        echo "Restarting $svc..."
        sudo systemctl restart "$svc"
    fi
done

# 5. Verify
echo "Waiting for startup..."
sleep 2
if curl -sf http://localhost:3002/api/health > /dev/null; then
    echo "=== Deploy successful! ==="
    echo "Soul v2 running at http://localhost:3002"
else
    echo "=== Deploy failed — health check failed ==="
    sudo journalctl -u soul-v2 --no-pager -n 20
    exit 1
fi
