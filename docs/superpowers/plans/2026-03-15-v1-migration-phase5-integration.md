# V1 Migration Phase 5: Integration & Deploy

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Final integration: verify all 13 servers build and run together, deploy systemd services, configure nginx, update CLAUDE.md, and run full verification.

**Architecture:** All 13 servers running concurrently on titan-pi/titan-pc, proxied through nginx, managed by systemd.

---

## Task 1: Makefile Finalization

**Files:** Modify `Makefile`

- [ ] **Step 1:** Verify all 13 build targets exist and compile:

```bash
make build-infra build-quality build-data build-docs build-sentinel build-bench build-mesh build-scout
```

- [ ] **Step 2:** Verify `make build` builds all 13 binaries + frontend.

- [ ] **Step 3:** Verify `make serve` starts all 13 servers.

- [ ] **Step 4:** Verify `make clean` removes all 13 binaries.

- [ ] **Step 5:** Commit any fixes: `fix: finalize Makefile for 13 servers`

---

## Task 2: SystemD Services

**Files:** 8 service files in `deploy/`

- [ ] **Step 1:** Verify all 8 new service files exist:

```
deploy/soul-v2-infra.service
deploy/soul-v2-quality.service
deploy/soul-v2-data.service
deploy/soul-v2-docs.service
deploy/soul-v2-sentinel.service
deploy/soul-v2-bench.service
deploy/soul-v2-mesh.service
deploy/soul-v2-scout.service
```

- [ ] **Step 2:** Install services:

```bash
sudo cp deploy/soul-v2-*.service /etc/systemd/system/
sudo systemctl daemon-reload
```

- [ ] **Step 3:** Enable and start all services:

```bash
for svc in infra quality data docs sentinel bench mesh scout; do
    sudo systemctl enable soul-v2-$svc
    sudo systemctl start soul-v2-$svc
done
```

- [ ] **Step 4:** Verify all services running:

```bash
for svc in infra quality data docs sentinel bench mesh scout; do
    systemctl is-active soul-v2-$svc
done
```

- [ ] **Step 5:** Check logs for errors:

```bash
for svc in infra quality data docs sentinel bench mesh scout; do
    journalctl -u soul-v2-$svc --no-pager -n 5
done
```

---

## Task 3: Nginx Configuration

**Files:** `/etc/nginx/sites-enabled/titan-services.conf`

- [ ] **Step 1:** Add proxy_pass entries for 8 new servers:

```nginx
location /api/infra/ {
    proxy_pass http://127.0.0.1:3012;
}
location /api/quality/ {
    proxy_pass http://127.0.0.1:3014;
}
location /api/data/ {
    proxy_pass http://127.0.0.1:3016;
}
location /api/docs/ {
    proxy_pass http://127.0.0.1:3018;
}
location /api/scout/ {
    proxy_pass http://127.0.0.1:3020;
}
location /api/sentinel/ {
    proxy_pass http://127.0.0.1:3022;
}
location /api/mesh/ {
    proxy_pass http://127.0.0.1:3024;
}
location /api/bench/ {
    proxy_pass http://127.0.0.1:3026;
}
```

- [ ] **Step 2:** Test nginx config: `sudo nginx -t`

- [ ] **Step 3:** Reload: `sudo systemctl reload nginx`

---

## Task 4: Health Check Sweep

- [ ] **Step 1:** Hit all 13 health endpoints:

```bash
for port in 3002 3004 3006 3008 3010 3012 3014 3016 3018 3020 3022 3024 3026; do
    echo "Port $port: $(curl -s http://127.0.0.1:$port/api/health | head -c 100)"
done
```

Expected: all return `{"status":"ok",...}`

- [ ] **Step 2:** Test chat product selector — set each of the 19 products via WebSocket and verify no errors.

- [ ] **Step 3:** Test tool dispatch — send a tool call to each product via chat and verify responses.

---

## Task 5: Update CLAUDE.md

**Files:** Modify `CLAUDE.md`

- [ ] **Step 1:** Update Architecture section with all 13 servers.

- [ ] **Step 2:** Update Environment Variables table with 8 new server entries.

- [ ] **Step 3:** Update Chat Product Routing section with new tool counts (93 total).

- [ ] **Step 4:** Commit: `docs: update CLAUDE.md for full v1 migration`

---

## Task 6: Full Verification

- [ ] **Step 1:** `make verify-static` → PASS

- [ ] **Step 2:** `go test -race -count=1 ./internal/... ./pkg/... -v` → ALL PASS

- [ ] **Step 3:** `npx tsc --noEmit` → PASS

- [ ] **Step 4:** `npx vite build` → SUCCESS

- [ ] **Step 5:** All 13 systemd services active

- [ ] **Step 6:** All health endpoints responding

- [ ] **Step 7:** Final commit: `feat: complete soul v1 → v2 full migration`

---

## Summary

| Task | What |
|------|------|
| 1 | Makefile finalization |
| 2 | SystemD service deployment |
| 3 | Nginx proxy configuration |
| 4 | Health check sweep |
| 5 | CLAUDE.md update |
| 6 | Full verification |
