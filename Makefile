.PHONY: build build-go build-tasks build-tutor build-projects build-observe build-infra build-quality build-data build-docs build-sentinel build-bench build-mesh build-scout web serve types clean deploy
.PHONY: verify verify-static verify-unit verify-integ verify-e2e verify-review
.PHONY: check-bundle check-secrets check-deps

# Build
build: web build-go build-tasks build-tutor build-projects build-observe build-infra build-quality build-data build-docs build-sentinel build-bench build-mesh build-scout
build-go:
	go build -o soul-chat ./cmd/chat
build-tasks:
	go build -o soul-tasks ./cmd/tasks
build-tutor:
	go build -o soul-tutor ./cmd/tutor
build-projects:
	go build -o soul-projects ./cmd/projects
build-observe:
	go build -o soul-observe ./cmd/observe
build-infra:
	go build -o soul-infra ./cmd/infra
build-quality:
	go build -o soul-quality ./cmd/quality
build-data:
	go build -o soul-data ./cmd/data
build-docs:
	go build -o soul-docs ./cmd/docs
build-sentinel:
	go build -o soul-sentinel ./cmd/sentinel
build-bench:
	go build -o soul-bench ./cmd/bench
build-mesh:
	go build -o soul-mesh ./cmd/mesh
build-scout:
	go build -o soul-scout ./cmd/scout
web:
	cd web && npx vite build
serve: build
	./soul-chat serve & ./soul-tasks serve & ./soul-tutor serve & ./soul-projects serve & ./soul-observe serve & ./soul-infra serve & ./soul-quality serve & ./soul-data serve & ./soul-docs serve & ./soul-sentinel serve & ./soul-bench serve & ./soul-mesh serve & ./soul-scout serve & wait
clean:
	rm -f soul-chat soul-tasks soul-tutor soul-projects soul-observe soul-infra soul-quality soul-data soul-docs soul-sentinel soul-bench soul-mesh soul-scout
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
	go test -race -count=1 ./internal/... ./pkg/...
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
