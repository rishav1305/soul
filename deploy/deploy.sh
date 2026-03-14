#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

echo "=== Soul v2 Deploy ==="

# 1. Build frontend
echo "Building frontend..."
cd "$PROJECT_DIR/web"
npm run build

# 2. Build Go binary
echo "Building Go binary..."
cd "$PROJECT_DIR"
go build -o soul-chat ./cmd/chat

# 3. Run tests
echo "Running tests..."
go test -race -count=1 ./...

# 4. Install systemd service
echo "Installing systemd service..."
sudo cp deploy/soul-v2.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable soul-v2
sudo systemctl restart soul-v2

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
