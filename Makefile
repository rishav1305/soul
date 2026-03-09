.PHONY: build build-go web serve types clean deploy
.PHONY: verify verify-static verify-unit verify-integ verify-e2e verify-review
.PHONY: check-bundle check-secrets check-deps

# Build
build: web build-go
build-go:
	go build -o soul ./cmd/soul
web:
	cd web && npx vite build
serve: build
	./soul serve
clean:
	rm -f soul
	rm -rf web/dist

# Generate
types:
	go run ./tools/specgen.go

# Verify (full stack)
verify: verify-static verify-unit verify-integ
verify-static: verify-static-go verify-static-ts check-secrets check-deps
verify-static-go:
	go vet ./...
	go build ./...
verify-static-ts:
	cd web && npx tsc --noEmit
verify-unit:
	go test -race -count=1 ./internal/...
verify-integ:
	go test -race -count=1 ./tests/integration/...

# Individual checks
check-bundle:
	@bash tests/verify/bundle_check.sh
check-secrets:
	@bash tests/verify/secret_scan.sh
check-deps:
	@echo "Checking Go vulnerabilities..."
	@if command -v govulncheck >/dev/null 2>&1; then govulncheck ./...; else echo "SKIP: govulncheck not installed"; fi
	@echo "Checking npm vulnerabilities..."
	@cd web && npm audit --audit-level=high 2>/dev/null || echo "WARN: npm audit found issues"

# Deploy to systemd
deploy:
	bash deploy/deploy.sh
