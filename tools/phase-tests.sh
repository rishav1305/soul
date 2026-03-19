#!/bin/bash
# Phase-specific verification tests
# Usage: bash tools/phase-tests.sh <phase>
# Phases: foundation, batch1, batch2, batch3, integration

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'
PASS=0
FAIL=0

check() {
    local desc="$1"
    local cmd="$2"
    if eval "$cmd" > /dev/null 2>&1; then
        echo -e "  ${GREEN}[PASS]${NC} $desc"
        PASS=$((PASS + 1))
    else
        echo -e "  ${RED}[FAIL]${NC} $desc"
        FAIL=$((FAIL + 1))
    fi
}

check_output() {
    local desc="$1"
    local cmd="$2"
    local result
    result=$(eval "$cmd" 2>&1) || true
    if [ -n "$result" ]; then
        echo -e "  ${GREEN}[PASS]${NC} $desc"
        echo -e "    ${YELLOW}-> $result${NC}"
        PASS=$((PASS + 1))
    else
        echo -e "  ${RED}[FAIL]${NC} $desc"
        FAIL=$((FAIL + 1))
    fi
}

summary() {
    echo ""
    echo "============================================"
    if [ $FAIL -eq 0 ]; then
        echo -e "  ${GREEN}ALL PASSED: $PASS/$((PASS + FAIL))${NC}"
    else
        echo -e "  ${RED}FAILED: $FAIL | PASSED: $PASS | TOTAL: $((PASS + FAIL))${NC}"
    fi
    echo "============================================"
    return $FAIL
}

phase_foundation() {
    echo ""
    echo "============================================"
    echo "  PHASE: FOUNDATION VERIFICATION"
    echo "============================================"
    echo ""

    echo "Schema Migration:"
    check "tier column exists" "sqlite3 ~/.soul-v2/scout.db 'PRAGMA table_info(leads)' | grep -q tier"
    check "contact_type column exists" "sqlite3 ~/.soul-v2/scout.db 'PRAGMA table_info(leads)' | grep -q contact_type"
    check "intent column exists" "sqlite3 ~/.soul-v2/scout.db 'PRAGMA table_info(leads)' | grep -q intent"
    check "warmth column exists" "sqlite3 ~/.soul-v2/scout.db 'PRAGMA table_info(leads)' | grep -q warmth"
    check "interaction_count column exists" "sqlite3 ~/.soul-v2/scout.db 'PRAGMA table_info(leads)' | grep -q interaction_count"
    check "channels column exists" "sqlite3 ~/.soul-v2/scout.db 'PRAGMA table_info(leads)' | grep -q channels"
    check "last_interaction_at column exists" "sqlite3 ~/.soul-v2/scout.db 'PRAGMA table_info(leads)' | grep -q last_interaction_at"
    check "source_ref_id column exists" "sqlite3 ~/.soul-v2/scout.db 'PRAGMA table_info(leads)' | grep -q source_ref_id"

    echo ""
    echo "New Tables:"
    check "lead_artifacts table exists" "sqlite3 ~/.soul-v2/scout.db '.tables' | grep -q lead_artifacts"
    check "interactions table exists" "sqlite3 ~/.soul-v2/scout.db '.tables' | grep -q interactions"
    check "content_posts table exists" "sqlite3 ~/.soul-v2/scout.db '.tables' | grep -q content_posts"
    check "content_backlog table exists" "sqlite3 ~/.soul-v2/scout.db '.tables' | grep -q content_backlog"

    echo ""
    echo "Pipeline Definitions:"
    check "job pipeline has qualified stage" "grep -q 'qualified' internal/scout/pipelines/pipelines.go"
    check "job pipeline has preparing stage" "grep -q 'preparing' internal/scout/pipelines/pipelines.go"
    check "job pipeline has skipped terminal" "grep -q 'skipped' internal/scout/pipelines/pipelines.go"
    check "freelance pipeline has proposal-ready" "grep -q 'proposal-ready' internal/scout/pipelines/pipelines.go"
    check "referral pipeline exists" "grep -q '\"referral\"' internal/scout/pipelines/pipelines.go"
    check "networking pipeline exists" "grep -q '\"networking\"' internal/scout/pipelines/pipelines.go"
    check "knownPipelines includes referral" "grep -q 'referral' internal/scout/store/analytics.go"
    check "knownPipelines includes networking" "grep -q 'networking' internal/scout/store/analytics.go"

    echo ""
    echo "ValidateTransition Enforcement:"
    check "server.go enforces ValidateTransition" "grep -q 'ValidateTransition' internal/scout/server/server.go"

    echo ""
    echo "Build and Tests:"
    check "go vet passes" "go vet ./internal/scout/..."
    check "go build passes" "go build ./internal/scout/..."
    check "pipeline tests pass" "go test -count=1 ./internal/scout/pipelines/..."
    check "store tests pass" "go test -count=1 ./internal/scout/store/..."

    summary
}

phase_batch1() {
    echo ""
    echo "============================================"
    echo "  PHASE: BATCH 1 VERIFICATION"
    echo "============================================"
    echo ""

    echo "AI Tool Files:"
    check "resume.go" "test -f internal/scout/ai/resume.go"
    check "resume_test.go" "test -f internal/scout/ai/resume_test.go"
    check "freelance_score.go" "test -f internal/scout/ai/freelance_score.go"
    check "freelance_score_test.go" "test -f internal/scout/ai/freelance_score_test.go"
    check "networking.go" "test -f internal/scout/ai/networking.go"
    check "networking_test.go" "test -f internal/scout/ai/networking_test.go"
    check "content_series.go" "test -f internal/scout/ai/content_series.go"
    check "content_topic.go" "test -f internal/scout/ai/content_topic.go"
    check "hook_writer.go" "test -f internal/scout/ai/hook_writer.go"
    check "expert_application.go" "test -f internal/scout/ai/expert_application.go"
    check "call_prep.go" "test -f internal/scout/ai/call_prep.go"

    echo ""
    echo "Runner:"
    check "runner/runner.go" "test -f internal/scout/runner/runner.go"
    check "runner/job.go" "test -f internal/scout/runner/job.go"
    check "runner/runner_test.go" "test -f internal/scout/runner/runner_test.go"

    echo ""
    echo "Tier Classifier:"
    check "sweep/classify.go" "test -f internal/scout/sweep/classify.go"
    check "sweep/classify_test.go" "test -f internal/scout/sweep/classify_test.go"
    check_output "dream companies count" "cat ~/.soul-v2/dream-companies.json | python3 -c 'import json,sys; print(str(len(json.load(sys.stdin))) + \" companies\")'"

    echo ""
    echo "Tool Registration:"
    check "resume tool in scout.go" "grep -qi 'resume' internal/chat/context/scout.go"
    check "freelance score in scout.go" "grep -qi 'freelance' internal/chat/context/scout.go"
    check "networking in scout.go" "grep -qi 'networking' internal/chat/context/scout.go"

    echo ""
    echo "Build and Tests:"
    check "go vet passes" "go vet ./internal/scout/..."
    check "go build passes" "go build ./internal/scout/..."
    check "AI tool tests pass" "go test -count=1 ./internal/scout/ai/..."
    check "runner tests pass" "go test -count=1 ./internal/scout/runner/..."
    check "sweep tests pass" "go test -count=1 ./internal/scout/sweep/..."
    check "race detector clean" "go test -race -count=1 ./internal/scout/runner/..."

    summary
}

phase_batch2() {
    echo ""
    echo "============================================"
    echo "  PHASE: BATCH 2 VERIFICATION"
    echo "============================================"
    echo ""

    echo "Batch 2 AI Tool Files:"
    check "sow.go" "test -f internal/scout/ai/sow.go"
    check "contract_followup.go" "test -f internal/scout/ai/contract_followup.go"
    check "case_study.go" "test -f internal/scout/ai/case_study.go"
    check "consulting_followup.go" "test -f internal/scout/ai/consulting_followup.go"
    check "advisory_proposal.go" "test -f internal/scout/ai/advisory_proposal.go"
    check "project_proposal.go" "test -f internal/scout/ai/project_proposal.go"
    check "upsell_evaluator.go" "test -f internal/scout/ai/upsell_evaluator.go"
    check "thread_converter.go" "test -f internal/scout/ai/thread_converter.go"
    check "substack_expander.go" "test -f internal/scout/ai/substack_expander.go"
    check "reactive_content.go" "test -f internal/scout/ai/reactive_content.go"
    check "engagement_reply.go" "test -f internal/scout/ai/engagement_reply.go"
    check "content_metrics.go" "test -f internal/scout/ai/content_metrics.go"
    check "linkedin_update.go" "test -f internal/scout/ai/linkedin_update.go"
    check "github_readme.go" "test -f internal/scout/ai/github_readme.go"
    check "profile_audit.go" "test -f internal/scout/ai/profile_audit.go"
    check "testimonial_request.go" "test -f internal/scout/ai/testimonial_request.go"
    check "pin_recommendation.go" "test -f internal/scout/ai/pin_recommendation.go"
    check "upsell.go" "test -f internal/scout/ai/upsell.go"

    echo ""
    echo "Runner Wiring:"
    check "runner/networking.go" "test -f internal/scout/runner/networking.go"
    check "runner/freelance.go" "test -f internal/scout/runner/freelance.go"
    check "runner/contracts.go" "test -f internal/scout/runner/contracts.go"
    check "runner/consulting.go" "test -f internal/scout/runner/consulting.go"
    check "runner/content.go" "test -f internal/scout/runner/content.go"
    check "runner/profile.go" "test -f internal/scout/runner/profile.go"

    echo ""
    echo "Tool Counts:"
    check_output "tool cases in server.go" "grep -c 'case \"' internal/scout/server/server.go | tr -d ' ' | xargs -I{} echo '{} cases'"
    check_output "tool defs in scout.go" "grep -c 'Name:' internal/chat/context/scout.go | tr -d ' ' | xargs -I{} echo '{} definitions'"

    echo ""
    echo "Build and Tests:"
    check "go vet passes" "go vet ./internal/scout/..."
    check "go build all passes" "go build ./..."
    check "ALL scout tests pass" "go test -count=1 ./internal/scout/..."
    check "race detector (runner)" "go test -race -count=1 ./internal/scout/runner/..."
    check "race detector (ai)" "go test -race -count=1 ./internal/scout/ai/..."
    check "scout binary builds" "go build -o /tmp/soul-scout-test ./cmd/scout && rm -f /tmp/soul-scout-test"

    summary
}

phase_batch3() {
    echo ""
    echo "============================================"
    echo "  PHASE: BATCH 3 FRONTEND VERIFICATION"
    echo "============================================"
    echo ""

    echo "Gate Components:"
    check "PriorityQueue.tsx" "test -f web/src/components/scout/PriorityQueue.tsx"
    check "GateAction.tsx" "test -f web/src/components/scout/GateAction.tsx"
    check "JobGate.tsx" "test -f web/src/components/scout/JobGate.tsx"
    check "FreelanceGate.tsx" "test -f web/src/components/scout/FreelanceGate.tsx"
    check "NetworkingGate.tsx" "test -f web/src/components/scout/NetworkingGate.tsx"
    check "ContentGate.tsx" "test -f web/src/components/scout/ContentGate.tsx"
    check "ConsultingGate.tsx" "test -f web/src/components/scout/ConsultingGate.tsx"
    check "ContractGate.tsx" "test -f web/src/components/scout/ContractGate.tsx"
    check "ProfileGate.tsx" "test -f web/src/components/scout/ProfileGate.tsx"
    check "MetricsDashboard.tsx" "test -f web/src/components/scout/MetricsDashboard.tsx"

    echo ""
    echo "Code Quality:"
    check "tsc --noEmit passes" "cd web && npx tsc --noEmit"
    check_output "data-testid count" "grep -r 'data-testid' web/src/components/scout/ 2>/dev/null | wc -l | tr -d ' ' | xargs -I{} echo '{} attributes'"
    check "no unsafe HTML usage" "! grep -r 'innerHTML' web/src/components/scout/ 2>/dev/null"
    check "zinc palette used" "grep -rq 'zinc' web/src/components/scout/"

    echo ""
    echo "ScoutPage Integration:"
    check "PriorityQueue in ScoutPage" "grep -qi 'priority' web/src/pages/ScoutPage.tsx"

    echo ""
    echo "Full Static Verify:"
    check "go vet passes" "go vet ./..."
    check "go build passes" "go build ./..."

    summary
}

phase_integration() {
    echo ""
    echo "============================================"
    echo "  PHASE: INTEGRATION VERIFICATION"
    echo "============================================"
    echo ""

    echo "Full Build (14 binaries):"
    for bin in chat tasks tutor projects observe mcp infra quality data docs sentinel bench mesh scout; do
        check "soul-$bin builds" "go build -o /tmp/soul-${bin}-test ./cmd/${bin} && rm -f /tmp/soul-${bin}-test"
    done

    echo ""
    echo "Full Verification:"
    check "go vet (all)" "go vet ./..."
    check "go build (all)" "go build ./..."
    check "tsc --noEmit" "cd web && npx tsc --noEmit"
    check "unit tests pass" "go test -count=1 ./internal/..."
    check "race detector (scout)" "go test -race -count=1 ./internal/scout/..."

    echo ""
    echo "Security Checks:"
    check "no hardcoded API keys in ai/" "! grep -rn 'sk-\|api_key\s*=\s*\"[a-zA-Z0-9]' internal/scout/ai/"
    check "no SQL concatenation in ai/" "! grep -rn 'fmt.Sprintf.*SELECT\|fmt.Sprintf.*INSERT\|fmt.Sprintf.*UPDATE' internal/scout/ai/"
    check "no direct anthropic import" "! grep -rn 'github.com/anthropic' internal/scout/ai/"

    echo ""
    echo "File Inventory:"
    check_output "AI tool files" "ls internal/scout/ai/*.go 2>/dev/null | grep -v _test.go | wc -l | tr -d ' ' | xargs -I{} echo '{} files (target: 35+)'"
    check_output "Runner phase files" "ls internal/scout/runner/*.go 2>/dev/null | grep -v _test.go | wc -l | tr -d ' ' | xargs -I{} echo '{} files'"
    check_output "Frontend gates" "ls web/src/components/scout/*Gate*.tsx web/src/components/scout/PriorityQueue.tsx web/src/components/scout/MetricsDashboard.tsx 2>/dev/null | wc -l | tr -d ' ' | xargs -I{} echo '{} components'"
    check_output "Test files" "find internal/scout/ -name '*_test.go' | wc -l | tr -d ' ' | xargs -I{} echo '{} test files'"

    echo ""
    echo "Config:"
    check "resume-baseline.md" "test -f ~/.soul-v2/resume-baseline.md"
    check "dream-companies.json" "test -f ~/.soul-v2/dream-companies.json"

    summary
}

case "${1:-}" in
    foundation) phase_foundation ;;
    batch1) phase_batch1 ;;
    batch2) phase_batch2 ;;
    batch3) phase_batch3 ;;
    integration) phase_integration ;;
    all) phase_foundation; phase_batch1; phase_batch2; phase_batch3; phase_integration ;;
    *) echo "Usage: bash tools/phase-tests.sh <foundation|batch1|batch2|batch3|integration|all>"; exit 1 ;;
esac
