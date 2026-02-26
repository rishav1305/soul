.PHONY: build dev clean test proto web build-go

VERSION := 0.2.0-alpha

# Build the full binary (React SPA + Go)
build: web
	go build -ldflags="-s -w -X main.version=$(VERSION)" -o dist/soul ./cmd/soul

# Build just Go (no frontend rebuild)
build-go:
	go build -ldflags="-s -w -X main.version=$(VERSION)" -o dist/soul ./cmd/soul

# Build React SPA
web:
	cd web && npm ci && npm run build

# Generate protobuf code
proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/soul/v1/product.proto

# Run Go tests
test:
	go test ./... -v

# Run all tests (Go + React)
test-all: test
	cd web && npm test

# Development mode (Go hot reload + Vite dev server)
dev:
	@echo "Start in two terminals:"
	@echo "  Terminal 1: cd web && npm run dev"
	@echo "  Terminal 2: go run ./cmd/soul serve --dev"

# Clean build artifacts
clean:
	rm -rf dist/ web/dist/
