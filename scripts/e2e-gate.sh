#!/usr/bin/env bash
# e2e-gate.sh — 6-tier Soul v2 deployment verification
#
# Runs on titan-pi. Validates build, services, routes, functional
# endpoints, proxies, and assets. Exit 0 only if all blocking tiers pass.
#
# Usage:
#   ./scripts/e2e-gate.sh                    # auto-reads token from systemd
#   SOUL_V2_AUTH_TOKEN=xxx ./scripts/e2e-gate.sh  # explicit token
#   ./scripts/e2e-gate.sh --tier 2           # run single tier
#
# Reference: pepper-e2e-deployment-gate-2026-03-26.md

set -euo pipefail

# ── Configuration ──────────────────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
BASE_URL="${SOUL_V2_URL:-http://localhost:3002}"
GO_BIN="${GO_BIN:-/usr/local/go/bin/go}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# Counters
PASS_COUNT=0
FAIL_COUNT=0
WARN_COUNT=0
BLOCKING_FAIL=0

# ── Helpers ────────────────────────────────────────────────────────────────

pass() { PASS_COUNT=$((PASS_COUNT + 1)); echo -e "  ${GREEN}PASS${NC} $1"; }
fail() { FAIL_COUNT=$((FAIL_COUNT + 1)); echo -e "  ${RED}FAIL${NC} $1"; }
fail_blocking() { FAIL_COUNT=$((FAIL_COUNT + 1)); BLOCKING_FAIL=$((BLOCKING_FAIL + 1)); echo -e "  ${RED}FAIL${NC} $1 ${RED}[BLOCKING]${NC}"; }
warn() { WARN_COUNT=$((WARN_COUNT + 1)); echo -e "  ${YELLOW}WARN${NC} $1"; }
info() { echo -e "  ${CYAN}INFO${NC} $1"; }

tier_header() {
    echo ""
    echo -e "${BOLD}═══ Tier $1: $2 ═══${NC}"
}

tier_result() {
    local tier="$1" name="$2" blocking="$3"
    local tier_fails=$FAIL_COUNT
    if [ "$blocking" = "blocking" ] && [ "$BLOCKING_FAIL" -gt 0 ]; then
        echo -e "  ${RED}▸ Tier $tier ($name): FAIL [BLOCKING — aborting]${NC}"
        return 1
    fi
    echo -e "  ${GREEN}▸ Tier $tier ($name): PASS${NC}"
    return 0
}

# Read auth token: env var > systemd environment > fail
resolve_auth_token() {
    if [ -n "${SOUL_V2_AUTH_TOKEN:-}" ]; then
        return 0
    fi

    # Try reading from systemd unit environment
    local svc_env
    svc_env=$(systemctl show soul-v2 --property=Environment 2>/dev/null || true)
    if [[ "$svc_env" =~ SOUL_V2_AUTH_TOKEN=([^ ]+) ]]; then
        SOUL_V2_AUTH_TOKEN="${BASH_REMATCH[1]}"
        return 0
    fi

    echo -e "${RED}ERROR: SOUL_V2_AUTH_TOKEN not set and not found in systemd environment${NC}"
    echo "Set it via: SOUL_V2_AUTH_TOKEN=xxx ./scripts/e2e-gate.sh"
    exit 1
}

auth_header() {
    echo "Authorization: Bearer ${SOUL_V2_AUTH_TOKEN}"
}

curl_auth() {
    curl -s -H "$(auth_header)" "$@" || true
}

curl_auth_status() {
    curl -s -o /dev/null -w "%{http_code}" -H "$(auth_header)" "$@" 2>/dev/null || echo "000"
}

# ── Tier 1: Build ──────────────────────────────────────────────────────────

tier1_build() {
    tier_header 1 "Build"
    local pre_fails=$BLOCKING_FAIL

    # Go vet + build
    if (cd "$PROJECT_ROOT" && "$GO_BIN" vet ./... 2>&1); then
        pass "go vet ./..."
    else
        fail_blocking "go vet ./..."
    fi

    if (cd "$PROJECT_ROOT" && "$GO_BIN" build ./... 2>&1); then
        pass "go build ./..."
    else
        fail_blocking "go build ./..."
    fi

    # TypeScript — use tsc -b (NOT --noEmit) to match production build
    if (cd "$PROJECT_ROOT/web" && npx tsc -b 2>&1); then
        pass "tsc -b (strict project build)"
    else
        fail_blocking "tsc -b (strict project build)"
    fi

    # Vite build
    local build_output
    build_output=$(cd "$PROJECT_ROOT/web" && npx vite build 2>&1)
    if [ $? -eq 0 ]; then
        local module_count
        module_count=$(echo "$build_output" | grep -oP '\d+ modules' | head -1 || echo "? modules")
        pass "vite build ($module_count)"
    else
        fail_blocking "vite build"
    fi

    if [ "$BLOCKING_FAIL" -gt "$pre_fails" ]; then
        tier_result 1 "Build" "blocking" || return 1
    fi
    tier_result 1 "Build" "ok"
}

# ── Tier 2: Service Health ─────────────────────────────────────────────────

tier2_services() {
    tier_header 2 "Service Health"
    local pre_fails=$BLOCKING_FAIL

    local services=("soul-v2:3002" "soul-v2-tasks:3004" "soul-v2-scout:3020" "soul-v2-tutor:3006")
    for entry in "${services[@]}"; do
        local svc="${entry%%:*}"
        local port="${entry##*:}"
        if systemctl is-active --quiet "$svc" 2>/dev/null; then
            pass "$svc active"
        else
            fail_blocking "$svc NOT active"
        fi
    done

    # Health endpoints
    local health_status
    health_status=$(curl_auth_status "${BASE_URL}/api/health" 2>/dev/null)
    if [ "$health_status" = "200" ]; then
        pass "/api/health → 200"
    elif [ "$health_status" = "401" ]; then
        # 401 means server is up but auth works — check unauthenticated health
        local unauth_status
        unauth_status=$(curl -s -o /dev/null -w "%{http_code}" "${BASE_URL}/health" 2>/dev/null || echo "000")
        if [ "$unauth_status" = "200" ]; then
            pass "/health → 200 (unauthed endpoint OK)"
        else
            fail_blocking "/api/health → $health_status (auth failed)"
        fi
    else
        fail_blocking "/api/health → $health_status"
    fi

    # Auth status
    local auth_body
    auth_body=$(curl_auth "${BASE_URL}/api/auth/status" 2>/dev/null)
    if echo "$auth_body" | grep -q '"connected"' 2>/dev/null; then
        pass "/api/auth/status → connected"
    elif echo "$auth_body" | grep -q '"state"' 2>/dev/null; then
        local state
        state=$(echo "$auth_body" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("state","unknown"))' 2>/dev/null || echo "unknown")
        warn "/api/auth/status → state=$state (not connected)"
    else
        warn "/api/auth/status → unexpected response"
    fi

    # Tutor health (dedicated endpoint, no auth)
    local tutor_health
    tutor_health=$(curl -s "http://localhost:3006/health" 2>/dev/null)
    if echo "$tutor_health" | grep -q '"ok"' 2>/dev/null; then
        local topics
        topics=$(echo "$tutor_health" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("topic_count",0))' 2>/dev/null || echo "?")
        pass "tutor health → ok ($topics topics)"
    else
        fail_blocking "tutor health → not ok"
    fi

    # Scout health
    local scout_health
    scout_health=$(curl -s "http://localhost:3020/health" 2>/dev/null)
    if echo "$scout_health" | grep -q '"ok"' 2>/dev/null; then
        pass "scout health → ok"
    else
        fail_blocking "scout health → not ok"
    fi

    if [ "$BLOCKING_FAIL" -gt "$pre_fails" ]; then
        tier_result 2 "Service Health" "blocking" || return 1
    fi
    tier_result 2 "Service Health" "ok"
}

# ── Tier 3: Route Rendering ───────────────────────────────────────────────

tier3_routes() {
    tier_header 3 "Route Rendering"
    local pre_fails=$BLOCKING_FAIL

    # SPA routes (served as index.html, no auth needed)
    local routes=("/" "/chat" "/tasks" "/tutor" "/projects" "/scout" "/sentinel" "/mesh" "/bench" "/observe")
    local route_pass=0
    local route_total=${#routes[@]}

    for route in "${routes[@]}"; do
        local status body_size
        status=$(curl -s -o /tmp/e2e-route-check -w "%{http_code}" "${BASE_URL}${route}" 2>/dev/null || echo "000")
        body_size=$(wc -c < /tmp/e2e-route-check 2>/dev/null || echo 0)

        if [ "$status" = "200" ] && [ "$body_size" -gt 100 ]; then
            ((route_pass++))
        else
            fail_blocking "GET $route → HTTP $status (${body_size}B)"
        fi
    done

    if [ "$route_pass" -eq "$route_total" ]; then
        pass "all $route_total base routes → HTTP 200 (>100B)"
    fi

    # Dynamic routes (need existing data — advisory)
    local dynamic_routes=("/dashboard")
    for route in "${dynamic_routes[@]}"; do
        local status
        status=$(curl -s -o /dev/null -w "%{http_code}" "${BASE_URL}${route}" 2>/dev/null)
        if [ "$status" = "200" ]; then
            pass "GET $route → HTTP $status"
        else
            warn "GET $route → HTTP $status (may need data)"
        fi
    done

    rm -f /tmp/e2e-route-check

    if [ "$BLOCKING_FAIL" -gt "$pre_fails" ]; then
        tier_result 3 "Route Rendering" "blocking" || return 1
    fi
    tier_result 3 "Route Rendering" "ok"
}

# ── Tier 4: Functional ────────────────────────────────────────────────────

tier4_functional() {
    tier_header 4 "Functional"
    local pre_fails=$BLOCKING_FAIL

    # List sessions
    local sessions_status
    sessions_status=$(curl_auth_status "${BASE_URL}/api/sessions" 2>/dev/null)
    if [ "$sessions_status" = "200" ]; then
        local session_count
        session_count=$(curl_auth "${BASE_URL}/api/sessions" 2>/dev/null | python3 -c 'import json,sys; d=json.load(sys.stdin); print(len(d) if isinstance(d,list) else "?")' 2>/dev/null || echo "?")
        pass "/api/sessions → 200 ($session_count sessions)"
    else
        fail_blocking "/api/sessions → $sessions_status"
    fi

    # Create session
    local create_response create_status create_body
    create_response=$(curl -s -H "$(auth_header)" -X POST -H "Content-Type: application/json" \
        -d '{"title":"e2e-gate-test"}' \
        -w "\n%{http_code}" "${BASE_URL}/api/sessions" 2>/dev/null || echo -e "\n000")
    create_status=$(echo "$create_response" | tail -1)
    create_body=$(echo "$create_response" | head -n -1)

    local test_session_id=""
    if [ "$create_status" = "201" ] || [ "$create_status" = "200" ]; then
        test_session_id=$(echo "$create_body" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("id",""))' 2>/dev/null || echo "")
        pass "POST /api/sessions → $create_status (id=${test_session_id:0:8}...)"
    else
        fail_blocking "POST /api/sessions → $create_status"
    fi

    # Delete session (cleanup)
    if [ -n "$test_session_id" ]; then
        local delete_status
        delete_status=$(curl -s -o /dev/null -w "%{http_code}" -H "$(auth_header)" -X DELETE "${BASE_URL}/api/sessions/${test_session_id}" 2>/dev/null || echo "000")
        if [ "$delete_status" = "200" ] || [ "$delete_status" = "204" ]; then
            pass "DELETE /api/sessions/$test_session_id → $delete_status"
        else
            warn "DELETE /api/sessions/$test_session_id → $delete_status"
        fi
    fi

    # WS ticket
    local ticket_response ticket_status ticket_body
    ticket_response=$(curl -s -H "$(auth_header)" -w "\n%{http_code}" "${BASE_URL}/api/ws-ticket" 2>/dev/null || echo -e "\n000")
    ticket_status=$(echo "$ticket_response" | tail -1)
    ticket_body=$(echo "$ticket_response" | head -n -1)

    if [ "$ticket_status" = "200" ]; then
        local ticket
        ticket=$(echo "$ticket_body" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("ticket",""))' 2>/dev/null || echo "")
        if [ -n "$ticket" ]; then
            pass "/api/ws-ticket → valid ticket"

            # WS upgrade with valid ticket (2s timeout — WS stays open)
            local ws_status
            ws_status=$(curl -s -o /dev/null -w "%{http_code}" --max-time 2 \
                -H "Connection: Upgrade" -H "Upgrade: websocket" \
                -H "Sec-WebSocket-Version: 13" -H "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==" \
                "${BASE_URL}/ws?ticket=${ticket}" 2>/dev/null || echo "000")
            # curl with --max-time may append extra chars; extract first 3 digits
            ws_status="${ws_status:0:3}"
            if [ "$ws_status" = "101" ]; then
                pass "/ws?ticket=valid → 101 Switching Protocols"
            else
                warn "/ws?ticket=valid → $ws_status (ticket may be consumed)"
            fi
        else
            fail_blocking "/api/ws-ticket → no ticket in response"
        fi
    else
        fail_blocking "/api/ws-ticket → $ticket_status"
    fi

    # WS reject with invalid ticket
    local ws_reject_status
    ws_reject_status=$(curl -s -o /dev/null -w "%{http_code}" --max-time 2 \
        -H "Connection: Upgrade" -H "Upgrade: websocket" \
        -H "Sec-WebSocket-Version: 13" -H "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==" \
        "${BASE_URL}/ws?ticket=invalid-ticket-12345" 2>/dev/null || echo "000")
    if [ "$ws_reject_status" = "401" ]; then
        pass "/ws?ticket=invalid → 401 (correctly rejected)"
    else
        fail_blocking "/ws?ticket=invalid → $ws_reject_status (expected 401)"
    fi

    if [ "$BLOCKING_FAIL" -gt "$pre_fails" ]; then
        tier_result 4 "Functional" "blocking" || return 1
    fi
    tier_result 4 "Functional" "ok"
}

# ── Tier 5: Proxy Verification ─────────────────────────────────────────────

tier5_proxies() {
    tier_header 5 "Proxy Verification (advisory)"

    # Tasks proxy
    local tasks_status
    tasks_status=$(curl_auth_status "${BASE_URL}/api/tasks" 2>/dev/null)
    if [ "$tasks_status" = "200" ]; then
        pass "tasks proxy /api/tasks → 200"
    else
        warn "tasks proxy /api/tasks → $tasks_status"
    fi

    # Scout proxy
    local scout_status
    scout_status=$(curl_auth_status "${BASE_URL}/api/scout/health" 2>/dev/null)
    if [ "$scout_status" = "200" ]; then
        pass "scout proxy /api/scout/health → 200"
    else
        # Try direct scout endpoint
        local direct_scout
        direct_scout=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:3020/health" 2>/dev/null)
        if [ "$direct_scout" = "200" ]; then
            warn "scout proxy → $scout_status (but direct :3020 → 200)"
        else
            warn "scout proxy → $scout_status, direct → $direct_scout"
        fi
    fi

    # Tutor proxy
    local tutor_status
    tutor_status=$(curl_auth_status "${BASE_URL}/api/tutor/dashboard" 2>/dev/null)
    if [ "$tutor_status" = "200" ]; then
        pass "tutor proxy /api/tutor/dashboard → 200"
    else
        warn "tutor proxy /api/tutor/dashboard → $tutor_status"
    fi

    # Tier 5 is advisory — never blocks
    tier_result 5 "Proxy Verification" "ok"
}

# ── Tier 6: Asset Verification ─────────────────────────────────────────────

tier6_assets() {
    tier_header 6 "Asset Verification"
    local pre_fails=$BLOCKING_FAIL

    # Check served HTML contains current build hashes
    local html
    html=$(curl -s "${BASE_URL}/" 2>/dev/null)

    # Extract JS and CSS filenames from build output
    local dist_dir="$PROJECT_ROOT/web/dist/assets"
    if [ -d "$dist_dir" ]; then
        local js_file css_file
        js_file=$(ls "$dist_dir"/index-*.js 2>/dev/null | head -1 | xargs basename 2>/dev/null || echo "")
        css_file=$(ls "$dist_dir"/index-*.css 2>/dev/null | head -1 | xargs basename 2>/dev/null || echo "")

        if [ -n "$js_file" ] && echo "$html" | grep -q "$js_file"; then
            pass "JS hash match: $js_file"
        elif [ -n "$js_file" ]; then
            fail_blocking "JS hash mismatch: $js_file not in served HTML"
        else
            warn "no JS file found in dist/"
        fi

        if [ -n "$css_file" ] && echo "$html" | grep -q "$css_file"; then
            pass "CSS hash match: $css_file"
        elif [ -n "$css_file" ]; then
            fail_blocking "CSS hash mismatch: $css_file not in served HTML"
        else
            warn "no CSS file found in dist/"
        fi
    else
        warn "dist/assets/ not found — skipping hash check (run tier 1 first)"
    fi

    # Journal check — zero errors in last 5 minutes
    local error_count
    error_count=$(journalctl -u soul-v2 --since "5 min ago" --priority=err --no-pager 2>/dev/null | grep -cv '^\-\- ' 2>/dev/null) || error_count=0
    if [ "$error_count" -eq 0 ] 2>/dev/null; then
        pass "journal: zero errors in last 5 min"
    else
        fail_blocking "journal: $error_count errors in last 5 min"
    fi

    # Check for panics/fatals
    local panic_count
    panic_count=$(journalctl -u soul-v2 --since "5 min ago" --no-pager 2>/dev/null | grep -ciE 'panic|fatal' 2>/dev/null) || panic_count=0
    if [ "$panic_count" -eq 0 ] 2>/dev/null; then
        pass "journal: zero panics/fatals in last 5 min"
    else
        fail_blocking "journal: $panic_count panics/fatals in last 5 min"
    fi

    if [ "$BLOCKING_FAIL" -gt "$pre_fails" ]; then
        tier_result 6 "Asset Verification" "blocking" || return 1
    fi
    tier_result 6 "Asset Verification" "ok"
}

# ── Main ───────────────────────────────────────────────────────────────────

main() {
    echo -e "${BOLD}Soul v2 — E2E Deployment Gate${NC}"
    echo -e "Target: ${BASE_URL}"
    echo -e "Time:   $(date '+%Y-%m-%d %H:%M:%S %Z')"
    echo ""

    resolve_auth_token
    info "auth token resolved (${#SOUL_V2_AUTH_TOKEN} chars)"

    local single_tier="${1:-}"

    # Run tiers (abort on blocking failure)
    if [ -z "$single_tier" ] || [ "$single_tier" = "--tier" -a "${2:-}" = "1" ]; then
        tier1_build || { summary; exit 1; }
        [ -n "$single_tier" ] && { summary; exit $BLOCKING_FAIL; }
    fi

    if [ -z "$single_tier" ] || [ "$single_tier" = "--tier" -a "${2:-}" = "2" ]; then
        tier2_services || { summary; exit 1; }
        [ -n "$single_tier" ] && { summary; exit $BLOCKING_FAIL; }
    fi

    if [ -z "$single_tier" ] || [ "$single_tier" = "--tier" -a "${2:-}" = "3" ]; then
        tier3_routes || { summary; exit 1; }
        [ -n "$single_tier" ] && { summary; exit $BLOCKING_FAIL; }
    fi

    if [ -z "$single_tier" ] || [ "$single_tier" = "--tier" -a "${2:-}" = "4" ]; then
        tier4_functional || { summary; exit 1; }
        [ -n "$single_tier" ] && { summary; exit $BLOCKING_FAIL; }
    fi

    if [ -z "$single_tier" ] || [ "$single_tier" = "--tier" -a "${2:-}" = "5" ]; then
        tier5_proxies
        [ -n "$single_tier" ] && { summary; exit $BLOCKING_FAIL; }
    fi

    if [ -z "$single_tier" ] || [ "$single_tier" = "--tier" -a "${2:-}" = "6" ]; then
        tier6_assets || { summary; exit 1; }
        [ -n "$single_tier" ] && { summary; exit $BLOCKING_FAIL; }
    fi

    summary
    exit $BLOCKING_FAIL
}

summary() {
    echo ""
    echo -e "${BOLD}═══ Summary ═══${NC}"
    echo -e "  PASS: ${GREEN}${PASS_COUNT}${NC}  FAIL: ${RED}${FAIL_COUNT}${NC}  WARN: ${YELLOW}${WARN_COUNT}${NC}"
    if [ "$BLOCKING_FAIL" -eq 0 ]; then
        echo -e "  ${GREEN}${BOLD}GATE: PASS${NC}"
    else
        echo -e "  ${RED}${BOLD}GATE: FAIL ($BLOCKING_FAIL blocking failures)${NC}"
    fi
}

main "$@"
