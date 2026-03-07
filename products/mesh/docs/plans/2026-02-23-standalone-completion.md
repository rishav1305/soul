# Soul-Mesh Standalone Completion Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make soul-mesh a fully standalone, end-to-end working mesh where agents can connect to a hub over WebSocket, send heartbeats, and have the hub track cluster resources.

**Architecture:** Refactor 3 legacy modules (node.py, election.py, transport.py) to use the new `nodes` table with `role` column instead of `mesh_nodes` with `is_hub`. Add WebSocket heartbeat endpoint to server.py so agents can actually connect. Add stale-node sweep. Add mDNS discovery. Verify with integration test.

**Tech Stack:** Python 3.11+, aiosqlite, websockets, FastAPI, PyJWT, zeroconf, structlog

---

### Task 1: Refactor node.py -- remove legacy `register()` method

The `register()` method (lines 121-183) creates its own `mesh_nodes` table via raw aiosqlite, bypassing MeshDB entirely. This conflicts with the new `nodes` table created by `db.ensure_tables()`. The new `hub.register_node()` replaces this functionality.

**Files:**
- Modify: `soul_mesh/node.py`

**Step 1: Remove the `register()` method**

Delete lines 121-183 from `soul_mesh/node.py` (the entire `async def register()` method). Keep everything else: `__init__`, `init`, `_load_or_create_id`, `capability_score`, `to_dict`, and the module-level helper functions (`_get_ram_mb`, `_get_storage_mb`, `_is_battery_powered`).

**Step 2: Run full test suite**

Run: `.venv/bin/python -m pytest tests/ -v --tb=short`
Expected: All 179 tests pass. No test file references `register()` (there is no `test_node.py`).

**Step 3: Commit**

```bash
git add soul_mesh/node.py
git commit -m "refactor: remove legacy register() from node.py -- replaced by hub.register_node()"
```

---

### Task 2: Refactor election.py -- use MeshDB + `nodes` table

The `HubElection.elect()` method (lines 133-189) opens its own raw aiosqlite connection and queries `mesh_nodes` with columns `capability` and `is_hub` that don't exist in the new schema. Refactor it to accept a `MeshDB` instance and query the `nodes` table using `role` and computing capability from stored hardware stats.

**Files:**
- Modify: `soul_mesh/election.py`
- Modify: `tests/test_election.py`

**Step 1: Write new test for `HubElection.elect()` with MeshDB**

Add to `tests/test_election.py` at the end:

```python
from soul_mesh.db import MeshDB


class TestHubElectionWithDB:
    """Tests for HubElection.elect() with real MeshDB."""

    @pytest.fixture
    async def db(self, tmp_path):
        db = MeshDB(str(tmp_path / "test.db"))
        await db.ensure_tables()
        return db

    def _make_node(self, node_id="local-1", name="local", cap=40.0):
        class MockNode:
            def __init__(self):
                self.id = node_id
                self.name = name
                self.is_hub = False
                self._cap = cap

            def capability_score(self):
                return self._cap

        return MockNode()

    async def test_elect_no_db_becomes_hub(self):
        node = self._make_node()
        election = HubElection(node)
        result = await election.elect(db=None)
        assert result == node.id
        assert node.is_hub is True

    async def test_elect_with_db_empty_becomes_hub(self, db):
        node = self._make_node()
        election = HubElection(node)
        result = await election.elect(db=db)
        assert result == node.id
        assert node.is_hub is True

    async def test_elect_with_db_picks_highest_capability(self, db):
        # Insert two online nodes with different hardware
        await db.upsert_node({"id": "node-a", "name": "weak", "ram_total_mb": 2048, "storage_total_gb": 100})
        await db.execute("UPDATE nodes SET status = 'online' WHERE id = 'node-a'")
        await db.upsert_node({"id": "node-b", "name": "strong", "ram_total_mb": 8192, "storage_total_gb": 500})
        await db.execute("UPDATE nodes SET status = 'online' WHERE id = 'node-b'")

        node = self._make_node(node_id="node-a", cap=10.0)
        election = HubElection(node)
        result = await election.elect(db=db)
        assert result == "node-b"
        assert node.is_hub is False

    async def test_elect_with_db_hysteresis(self, db):
        # Current hub with moderate hardware
        await db.upsert_node({"id": "hub-1", "name": "current", "ram_total_mb": 4096, "storage_total_gb": 200})
        await db.execute("UPDATE nodes SET status = 'online', role = 'hub' WHERE id = 'hub-1'")
        # Challenger slightly better but within 20% margin
        await db.upsert_node({"id": "chal-1", "name": "challenger", "ram_total_mb": 4500, "storage_total_gb": 220})
        await db.execute("UPDATE nodes SET status = 'online' WHERE id = 'chal-1'")

        node = self._make_node(node_id="hub-1", cap=28.0)
        election = HubElection(node)
        result = await election.elect(db=db)
        # Hub should keep role due to hysteresis
        assert result == "hub-1"
```

**Step 2: Run test to verify it fails**

Run: `.venv/bin/python -m pytest tests/test_election.py::TestHubElectionWithDB -v --tb=short`
Expected: FAIL -- `elect()` still uses raw aiosqlite with `mesh_nodes`.

**Step 3: Rewrite `HubElection.elect()`**

Replace the `elect()` method in `soul_mesh/election.py` (lines 133-189). The new signature changes `db_path: str | None = None` to `db: MeshDB | None = None` (import MeshDB from soul_mesh.db). The method should:

1. If `db is None`, set local as hub, return local id (same as before)
2. Fetch all online nodes: `SELECT id, name, role, ram_total_mb, storage_total_gb FROM nodes WHERE status = 'online'`
3. Compute capability for each: `min(ram_total_mb / 8192, 1.0) * 40 + min(storage_total_gb / 500, 1.0) * 20`
4. Mark current hub by checking `role = 'hub'` instead of `is_hub = 1`
5. Call `elect_hub()` with the computed list
6. Update the winning node: `UPDATE nodes SET role = 'hub' WHERE id = ?` and reset others: `UPDATE nodes SET role = 'agent' WHERE id != ? AND role = 'hub'`
7. Update `self._local.is_hub` as before

```python
    async def elect(self, db=None) -> str:
        """Run election with optional MeshDB persistence.

        Parameters
        ----------
        db : MeshDB | None
            Database instance. If None, uses only the local node.
        """
        if db is None:
            self._local.is_hub = True
            return self._local.id

        rows = await db.fetch_all(
            "SELECT id, name, role, ram_total_mb, storage_total_gb "
            "FROM nodes WHERE status = 'online'"
        )

        if not rows:
            self._local.is_hub = True
            return self._local.id

        all_nodes = []
        for row in rows:
            ram = row.get("ram_total_mb", 0)
            storage = row.get("storage_total_gb", 0)
            cap = min(ram / 8192, 1.0) * 40 + min(storage / 500, 1.0) * 20
            all_nodes.append({
                "id": row["id"],
                "name": row.get("name", ""),
                "capability": round(cap, 2),
                "is_hub": row.get("role") == "hub",
            })

        winner_id = elect_hub(all_nodes)

        await db.execute(
            "UPDATE nodes SET role = 'agent' WHERE role = 'hub' AND id != ?",
            (winner_id,),
        )
        await db.execute(
            "UPDATE nodes SET role = 'hub' WHERE id = ?",
            (winner_id,),
        )

        was_hub = self._local.is_hub
        self._local.is_hub = winner_id == self._local.id

        if was_hub != self._local.is_hub:
            if self._local.is_hub:
                logger.info("This node elected as hub", capability=self._local.capability_score())
            else:
                logger.info("Hub role transferred", new_hub=winner_id[:8])

        return winner_id
```

Also remove the `import aiosqlite` line from the method (it was a lazy import inside the old elect).

**Step 4: Run tests**

Run: `.venv/bin/python -m pytest tests/test_election.py -v --tb=short`
Expected: All tests pass (15 original + 4 new = 19).

**Step 5: Run full suite**

Run: `.venv/bin/python -m pytest tests/ -v --tb=short`
Expected: All pass.

**Step 6: Commit**

```bash
git add soul_mesh/election.py tests/test_election.py
git commit -m "refactor: election.elect() uses MeshDB + nodes table instead of raw aiosqlite + mesh_nodes"
```

---

### Task 3: Refactor transport.py -- use `nodes` table with `role` column

Two methods in `transport.py` query `mesh_nodes`:
- `start()` line 53-54: `SELECT id, host, port FROM mesh_nodes WHERE id != ? AND status = 'online'`
- `send_to_hub()` line 177-178: `SELECT id FROM mesh_nodes WHERE is_hub = 1 AND status = 'online'`

Both need to query `nodes` instead, and `send_to_hub` needs `role = 'hub'` instead of `is_hub = 1`.

**Files:**
- Modify: `soul_mesh/transport.py`
- Modify: `tests/test_transport.py`

**Step 1: Update transport.py**

In `soul_mesh/transport.py`:

Line 53-54, change:
```python
        peers = await self._db.fetch_all(
            "SELECT id, host, port FROM mesh_nodes "
            "WHERE id != ? AND status = 'online'",
            (self._local.id,),
        )
```
to:
```python
        peers = await self._db.fetch_all(
            "SELECT id, host, port FROM nodes "
            "WHERE id != ? AND status = 'online'",
            (self._local.id,),
        )
```

Lines 177-178, change:
```python
        hub = await self._db.fetch_one(
            "SELECT id FROM mesh_nodes WHERE is_hub = 1 AND status = 'online'"
        )
```
to:
```python
        hub = await self._db.fetch_one(
            "SELECT id FROM nodes WHERE role = 'hub' AND status = 'online'"
        )
```

**Step 2: Update MockDB in test_transport.py**

The `MockDB.fetch_one()` checks `n.get("is_hub")`. Update it to check `n.get("role") == "hub"` instead. And `add_node()` should take `role="agent"` instead of `is_hub=False`.

In `tests/test_transport.py`:

Update `MockDB`:
```python
class MockDB:
    """Mock MeshDB for testing transport without real SQLite."""

    def __init__(self):
        self._nodes: list[dict] = []

    async def fetch_all(self, sql, params=()):
        return [n for n in self._nodes if n["id"] != params[0]] if params else self._nodes

    async def fetch_one(self, sql, params=()):
        for n in self._nodes:
            if n.get("role") == "hub" and n.get("status") == "online":
                return n
        return None

    def add_node(self, node_id, host="127.0.0.1", port=8340, role="agent"):
        self._nodes.append({
            "id": node_id, "host": host, "port": port,
            "role": role, "status": "online",
        })
```

Update `test_send_to_hub` (line 197): change `db.add_node("hub-1", is_hub=True)` to `db.add_node("hub-1", role="hub")`.

**Step 3: Run tests**

Run: `.venv/bin/python -m pytest tests/test_transport.py -v --tb=short`
Expected: All 18 tests pass.

**Step 4: Run full suite**

Run: `.venv/bin/python -m pytest tests/ -v --tb=short`
Expected: All pass.

**Step 5: Commit**

```bash
git add soul_mesh/transport.py tests/test_transport.py
git commit -m "refactor: transport uses nodes table with role column instead of mesh_nodes with is_hub"
```

---

### Task 4: Add WebSocket heartbeat endpoint to server.py

This is the critical missing piece. Agents connect to `ws://<hub>/api/mesh/ws?token=<jwt>` and send heartbeat JSON. The hub validates the token, then calls `hub.register_node()` on first message and `hub.process_heartbeat()` on subsequent messages.

**Files:**
- Modify: `soul_mesh/server.py`
- Modify: `tests/test_server.py`

**Step 1: Write failing tests**

Add to `tests/test_server.py`:

```python
import json
from unittest.mock import AsyncMock, patch

from soul_mesh.auth import create_mesh_token


class TestWebSocketEndpoint:
    """Tests for the /api/mesh/ws WebSocket heartbeat endpoint."""

    @pytest.fixture
    async def db(self, tmp_path):
        db = MeshDB(str(tmp_path / "test.db"))
        await db.ensure_tables()
        return db

    @pytest.fixture
    def app(self, db):
        return create_app(db, secret="test-secret-key-32-bytes-long!!!")

    async def test_ws_rejects_missing_token(self, app):
        from starlette.testclient import TestClient
        client = TestClient(app)
        with pytest.raises(Exception):
            with client.websocket_connect("/api/mesh/ws"):
                pass

    async def test_ws_rejects_invalid_token(self, app):
        from starlette.testclient import TestClient
        client = TestClient(app)
        with pytest.raises(Exception):
            with client.websocket_connect("/api/mesh/ws?token=bad-token"):
                pass

    async def test_ws_accepts_valid_token_and_heartbeat(self, app, db):
        from starlette.testclient import TestClient

        token = create_mesh_token("node-1", "acct-1", "test-secret-key-32-bytes-long!!!")
        client = TestClient(app)
        with client.websocket_connect(f"/api/mesh/ws?token={token}") as ws:
            heartbeat = {
                "node_id": "node-1",
                "name": "test-pi",
                "host": "10.0.0.5",
                "port": 8340,
                "platform": "linux",
                "arch": "aarch64",
                "cpu": {"cores": 4, "usage_percent": 15.0, "load_avg_1m": 0.5},
                "memory": {"total_mb": 4096, "available_mb": 2048, "used_percent": 50.0},
                "storage": {"mounts": [{"path": "/", "total_gb": 64, "free_gb": 30}]},
            }
            ws.send_json(heartbeat)
            response = ws.receive_json()
            assert response["status"] == "ok"

        # Verify node was registered in DB
        nodes = await db.fetch_all("SELECT * FROM nodes WHERE id = 'node-1'")
        assert len(nodes) == 1
        assert nodes[0]["name"] == "test-pi"
        assert nodes[0]["status"] == "online"
        assert nodes[0]["cpu_cores"] == 4

    async def test_ws_multiple_heartbeats(self, app, db):
        from starlette.testclient import TestClient

        token = create_mesh_token("node-2", "acct-1", "test-secret-key-32-bytes-long!!!")
        client = TestClient(app)
        with client.websocket_connect(f"/api/mesh/ws?token={token}") as ws:
            heartbeat = {
                "node_id": "node-2",
                "name": "pi",
                "host": "10.0.0.6",
                "port": 8340,
                "platform": "linux",
                "arch": "aarch64",
                "cpu": {"cores": 4, "usage_percent": 10.0, "load_avg_1m": 0.3},
                "memory": {"total_mb": 2048, "available_mb": 1024, "used_percent": 50.0},
                "storage": {"mounts": [{"path": "/", "total_gb": 32, "free_gb": 20}]},
            }
            ws.send_json(heartbeat)
            ws.receive_json()
            # Second heartbeat
            heartbeat["cpu"]["usage_percent"] = 50.0
            ws.send_json(heartbeat)
            resp2 = ws.receive_json()
            assert resp2["status"] == "ok"

        # Should have 2 heartbeat rows
        heartbeats = await db.fetch_all("SELECT * FROM heartbeats WHERE node_id = 'node-2'")
        assert len(heartbeats) == 2
```

**Step 2: Run tests to verify failure**

Run: `.venv/bin/python -m pytest tests/test_server.py::TestWebSocketEndpoint -v --tb=short`
Expected: FAIL -- no `/api/mesh/ws` route exists.

**Step 3: Implement WebSocket endpoint**

Update `soul_mesh/server.py`. The `create_app` function gains an optional `secret` parameter for JWT verification. Add a WebSocket route:

```python
def create_app(db: MeshDB, node: NodeInfo | None = None, secret: str = ""):
    from fastapi import FastAPI, WebSocket, WebSocketDisconnect, Query

    app = FastAPI(title="soul-mesh", version="0.2.0")
    app.state.db = db
    app.state.hub = Hub(db)
    app.state.node = node
    app.state.secret = secret

    # ... existing REST routes ...

    @app.websocket("/api/mesh/ws")
    async def ws_heartbeat(websocket: WebSocket, token: str = Query(default="")):
        """Accept agent WebSocket connections and process heartbeats."""
        from soul_mesh.auth import verify_mesh_token

        if not token or not app.state.secret:
            await websocket.close(code=4001, reason="missing token or secret")
            return

        try:
            claims = verify_mesh_token(token, app.state.secret)
        except Exception:
            await websocket.close(code=4003, reason="invalid token")
            return

        await websocket.accept()
        node_id = claims.get("node_id", "")
        hub = app.state.hub
        registered = False

        try:
            while True:
                data = await websocket.receive_json()
                if not registered:
                    await hub.register_node(data)
                    registered = True
                else:
                    await hub.process_heartbeat(node_id, data)
                await websocket.send_json({"status": "ok"})
        except WebSocketDisconnect:
            pass
        except Exception as exc:
            logger.warning("ws_error", node_id=node_id[:8], error=str(exc))
```

Add `import structlog` and `logger = structlog.get_logger("soul-mesh.server")` at the top of the module.

**Step 4: Update existing test fixture**

The existing tests in `TestServerApp` use `create_app(db)` which still works (secret defaults to ""). No changes needed to existing tests.

**Step 5: Run tests**

Run: `.venv/bin/python -m pytest tests/test_server.py -v --tb=short`
Expected: All tests pass (7 original + 4 new = 11).

**Step 6: Run full suite**

Run: `.venv/bin/python -m pytest tests/ -v --tb=short`
Expected: All pass.

**Step 7: Commit**

```bash
git add soul_mesh/server.py tests/test_server.py
git commit -m "feat: add WebSocket heartbeat endpoint -- agents can now connect to hub"
```

---

### Task 5: Add stale-node sweep background task

The hub needs a background task that periodically calls `hub.mark_stale_nodes()` to detect agents that stopped heartbeating. This runs as a FastAPI lifespan task.

**Files:**
- Modify: `soul_mesh/server.py`
- Modify: `tests/test_server.py`

**Step 1: Write failing test**

Add to `tests/test_server.py`:

```python
import asyncio
from datetime import datetime, timezone, timedelta


class TestStaleSweep:
    @pytest.fixture
    async def db(self, tmp_path):
        db = MeshDB(str(tmp_path / "test.db"))
        await db.ensure_tables()
        return db

    async def test_stale_sweep_marks_old_nodes(self, db):
        """Nodes with old heartbeats get marked stale by the sweep."""
        from soul_mesh.hub import Hub

        hub = Hub(db)
        # Register a node then manually set its heartbeat to 60s ago
        await db.upsert_node({"id": "old-node", "name": "stale-pi"})
        old_time = (datetime.now(timezone.utc) - timedelta(seconds=60)).strftime("%Y-%m-%dT%H:%M:%SZ")
        await db.execute("UPDATE nodes SET status = 'online', last_heartbeat = ? WHERE id = 'old-node'", (old_time,))

        stale = await hub.mark_stale_nodes(timeout_seconds=30)
        assert "old-node" in stale

        row = await db.fetch_one("SELECT status FROM nodes WHERE id = 'old-node'")
        assert row["status"] == "stale"
```

**Step 2: Run test to verify it passes**

This test uses Hub.mark_stale_nodes() directly which already works. It verifies the building block.

Run: `.venv/bin/python -m pytest tests/test_server.py::TestStaleSweep -v --tb=short`
Expected: PASS (mark_stale_nodes already works).

**Step 3: Add lifespan with stale sweep to server.py**

Update `create_app` to use a FastAPI lifespan that runs the sweep loop:

```python
def create_app(db: MeshDB, node: NodeInfo | None = None, secret: str = "", stale_interval: int = 30):
    from contextlib import asynccontextmanager
    from fastapi import FastAPI, WebSocket, WebSocketDisconnect, Query

    @asynccontextmanager
    async def lifespan(app):
        task = asyncio.create_task(_stale_sweep_loop(app.state.hub, stale_interval))
        yield
        task.cancel()
        try:
            await task
        except asyncio.CancelledError:
            pass

    app = FastAPI(title="soul-mesh", version="0.2.0", lifespan=lifespan)
    # ... rest of setup ...
```

Add the sweep loop as a module-level async function:

```python
async def _stale_sweep_loop(hub: Hub, interval: int) -> None:
    """Periodically mark nodes with no recent heartbeat as stale."""
    while True:
        try:
            await asyncio.sleep(interval)
            await hub.mark_stale_nodes(timeout_seconds=interval)
        except asyncio.CancelledError:
            return
        except Exception as exc:
            logger.warning("stale_sweep_error", error=str(exc))
```

Add `import asyncio` to the top of server.py.

**Step 4: Run full test suite**

Run: `.venv/bin/python -m pytest tests/ -v --tb=short`
Expected: All pass. The lifespan task starts/stops cleanly in tests (httpx AsyncClient handles it).

**Step 5: Commit**

```bash
git add soul_mesh/server.py tests/test_server.py
git commit -m "feat: add stale-node sweep background task to hub server"
```

---

### Task 6: Add mDNS discovery to discovery.py

The current `discovery.py` only supports Tailscale. Add mDNS/Zeroconf as the primary LAN discovery method using the `zeroconf` library (already in dependencies). Service type: `_soul-mesh._tcp.local.`

**Files:**
- Modify: `soul_mesh/discovery.py`
- Create: `tests/test_discovery.py`

**Step 1: Write failing tests**

Create `tests/test_discovery.py`:

```python
"""Tests for mesh peer discovery -- mDNS and Tailscale."""

from __future__ import annotations

import asyncio
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from soul_mesh.discovery import PeerDiscovery, MdnsAnnouncer


class MockNode:
    def __init__(self, node_id="local-1", name="test-node", port=8340):
        self.id = node_id
        self.name = name
        self.host = ""
        self.port = port
        self.account_id = "acct-1"

    def capability_score(self):
        return 30.0


class TestMdnsAnnouncer:
    def test_init(self):
        node = MockNode()
        announcer = MdnsAnnouncer(node)
        assert announcer._node is node

    def test_service_info_type(self):
        node = MockNode()
        announcer = MdnsAnnouncer(node)
        info = announcer._build_service_info("192.168.1.10")
        assert info is not None

    async def test_start_stop(self):
        node = MockNode()
        announcer = MdnsAnnouncer(node)
        with patch("soul_mesh.discovery.Zeroconf") as mock_zc:
            mock_instance = MagicMock()
            mock_zc.return_value = mock_instance
            await announcer.start("192.168.1.10")
            await announcer.stop()


class TestPeerDiscoveryInit:
    def test_defaults(self):
        node = MockNode()
        discovery = PeerDiscovery(local_node=node)
        assert discovery._discovery_interval == 30

    def test_custom_interval(self):
        node = MockNode()
        discovery = PeerDiscovery(local_node=node, discovery_interval=10)
        assert discovery._discovery_interval == 10

    def test_peers_starts_empty(self):
        node = MockNode()
        discovery = PeerDiscovery(local_node=node)
        assert discovery.peers == {}

    def test_get_online_peers_empty(self):
        node = MockNode()
        discovery = PeerDiscovery(local_node=node)
        assert discovery.get_online_peers() == []


class TestPeerDiscoveryOperations:
    def test_mark_peer_offline(self):
        node = MockNode()
        discovery = PeerDiscovery(local_node=node)
        discovery._peers["peer-1"] = {"id": "peer-1", "status": "online"}
        discovery.mark_peer_offline("peer-1")
        assert discovery._peers["peer-1"]["status"] == "offline"

    def test_mark_unknown_peer_offline_noop(self):
        node = MockNode()
        discovery = PeerDiscovery(local_node=node)
        discovery.mark_peer_offline("unknown")  # should not raise

    async def test_start_stop(self):
        node = MockNode()
        discovery = PeerDiscovery(local_node=node, discovery_interval=1)
        with patch.object(discovery, "_scan_peers", new_callable=AsyncMock):
            await discovery.start()
            assert discovery._running is True
            await discovery.stop()
            assert discovery._running is False

    def test_compute_capability(self):
        node = MockNode()
        discovery = PeerDiscovery(local_node=node)
        # 8192 MB RAM = max 40 pts, 500 GB storage = max 20 pts
        assert discovery._compute_capability(8192, 512000) == 60.0
        assert discovery._compute_capability(0, 0) == 0.0
        # 4096 MB RAM = 20 pts, 250 GB storage = ~10 pts
        cap = discovery._compute_capability(4096, 256000)
        assert 25 < cap < 35
```

**Step 2: Run tests to verify failure**

Run: `.venv/bin/python -m pytest tests/test_discovery.py -v --tb=short`
Expected: FAIL -- `MdnsAnnouncer` doesn't exist yet.

**Step 3: Add MdnsAnnouncer class to discovery.py**

Add to `soul_mesh/discovery.py`:

```python
import socket

try:
    from zeroconf import ServiceInfo, Zeroconf
    _HAS_ZEROCONF = True
except ImportError:
    _HAS_ZEROCONF = False


SERVICE_TYPE = "_soul-mesh._tcp.local."


class MdnsAnnouncer:
    """Announce this node on the LAN via mDNS/Zeroconf."""

    def __init__(self, node) -> None:
        self._node = node
        self._zc: Zeroconf | None = None
        self._info: ServiceInfo | None = None

    def _build_service_info(self, ip: str) -> ServiceInfo | None:
        if not _HAS_ZEROCONF:
            return None
        return ServiceInfo(
            SERVICE_TYPE,
            f"{self._node.name}.{SERVICE_TYPE}",
            addresses=[socket.inet_aton(ip)],
            port=self._node.port,
            properties={
                "node_id": self._node.id,
                "name": self._node.name,
            },
        )

    async def start(self, ip: str) -> None:
        if not _HAS_ZEROCONF:
            logger.warning("zeroconf not installed -- mDNS disabled")
            return
        self._info = self._build_service_info(ip)
        if not self._info:
            return
        self._zc = Zeroconf()
        self._zc.register_service(self._info)
        logger.info("mDNS announced", service=SERVICE_TYPE, ip=ip, port=self._node.port)

    async def stop(self) -> None:
        if self._zc and self._info:
            self._zc.unregister_service(self._info)
            self._zc.close()
            self._zc = None
        logger.debug("mDNS stopped")
```

**Step 4: Run tests**

Run: `.venv/bin/python -m pytest tests/test_discovery.py -v --tb=short`
Expected: All pass (~10 tests).

**Step 5: Run full suite**

Run: `.venv/bin/python -m pytest tests/ -v --tb=short`
Expected: All pass.

**Step 6: Commit**

```bash
git add soul_mesh/discovery.py tests/test_discovery.py
git commit -m "feat: add mDNS announcer for LAN discovery via Zeroconf"
```

---

### Task 7: Integration test -- agent connects to hub end-to-end

A single test that starts a hub server, creates an agent, has the agent send a heartbeat, and verifies the hub registered it.

**Files:**
- Create: `tests/test_integration.py`

**Step 1: Write the test**

```python
"""Integration test -- agent heartbeat -> hub registration."""

from __future__ import annotations

import asyncio
import json

import pytest

from soul_mesh.auth import create_mesh_token
from soul_mesh.config import MeshConfig
from soul_mesh.db import MeshDB
from soul_mesh.hub import Hub
from soul_mesh.node import NodeInfo
from soul_mesh.server import create_app


class TestAgentHubIntegration:
    """End-to-end: agent sends heartbeat over WebSocket, hub registers it."""

    @pytest.fixture
    async def db(self, tmp_path):
        db = MeshDB(str(tmp_path / "test.db"))
        await db.ensure_tables()
        return db

    async def test_heartbeat_registers_node(self, db):
        """Send a heartbeat via WebSocket and verify node appears in DB."""
        secret = "integration-test-secret-32-bytes!"
        app = create_app(db, secret=secret)

        from starlette.testclient import TestClient

        token = create_mesh_token("agent-1", "acct-1", secret)
        client = TestClient(app)

        with client.websocket_connect(f"/api/mesh/ws?token={token}") as ws:
            ws.send_json({
                "node_id": "agent-1",
                "name": "my-pi",
                "host": "192.168.1.50",
                "port": 8340,
                "platform": "linux",
                "arch": "aarch64",
                "cpu": {"cores": 4, "usage_percent": 12.0, "load_avg_1m": 0.3},
                "memory": {"total_mb": 4096, "available_mb": 2048, "used_percent": 50.0},
                "storage": {"mounts": [{"path": "/", "total_gb": 64, "free_gb": 40}]},
            })
            resp = ws.receive_json()
            assert resp["status"] == "ok"

        # Verify in DB
        nodes = await db.fetch_all("SELECT * FROM nodes")
        assert len(nodes) == 1
        assert nodes[0]["id"] == "agent-1"
        assert nodes[0]["name"] == "my-pi"
        assert nodes[0]["cpu_cores"] == 4
        assert nodes[0]["ram_total_mb"] == 4096
        assert nodes[0]["status"] == "online"

        # Verify heartbeat recorded
        heartbeats = await db.fetch_all("SELECT * FROM heartbeats WHERE node_id = 'agent-1'")
        assert len(heartbeats) == 1

    async def test_cluster_totals_after_heartbeats(self, db):
        """Two agents heartbeat, cluster totals reflect both."""
        secret = "integration-test-secret-32-bytes!"
        app = create_app(db, secret=secret)

        from starlette.testclient import TestClient
        client = TestClient(app)

        for i, (cores, ram) in enumerate([(4, 4096), (8, 16384)], start=1):
            token = create_mesh_token(f"agent-{i}", "acct-1", secret)
            with client.websocket_connect(f"/api/mesh/ws?token={token}") as ws:
                ws.send_json({
                    "node_id": f"agent-{i}",
                    "name": f"node-{i}",
                    "host": f"10.0.0.{i}",
                    "port": 8340,
                    "platform": "linux",
                    "arch": "x86_64",
                    "cpu": {"cores": cores, "usage_percent": 10.0, "load_avg_1m": 0.1},
                    "memory": {"total_mb": ram, "available_mb": ram // 2, "used_percent": 50.0},
                    "storage": {"mounts": [{"path": "/", "total_gb": 500, "free_gb": 250}]},
                })
                ws.receive_json()

        hub = Hub(db)
        totals = await hub.cluster_totals()
        assert totals["nodes_online"] == 2
        assert totals["cpu_cores"] == 12
        assert totals["ram_total_mb"] == 20480
```

**Step 2: Run integration tests**

Run: `.venv/bin/python -m pytest tests/test_integration.py -v --tb=short`
Expected: All pass (2 tests).

**Step 3: Run full suite**

Run: `.venv/bin/python -m pytest tests/ -v --tb=short`
Expected: All pass.

**Step 4: Commit**

```bash
git add tests/test_integration.py
git commit -m "test: add integration tests -- agent heartbeat to hub end-to-end"
```

---

### Task 8: Final verification and push

**Step 1: Run full test suite**

Run: `.venv/bin/python -m pytest tests/ -v --tb=short`
Expected: All tests pass. Count should be ~200+.

**Step 2: Verify no brain imports**

Run: `grep -r "from brain" soul_mesh/`
Expected: No output.

**Step 3: Verify no mesh_nodes references in source**

Run: `grep -rn "mesh_nodes" soul_mesh/`
Expected: No output (zero matches in source code).

**Step 4: Verify pip install and imports**

Run: `.venv/bin/pip install -e ".[server]" && .venv/bin/python -c "from soul_mesh import Hub, Agent, MeshConfig, MeshTransport, HubElection; print('OK')"`
Expected: `OK`

**Step 5: Verify CLI**

Run: `.venv/bin/soul-mesh --help`
Expected: Shows init, serve, status, nodes.

**Step 6: Push to Gitea**

```bash
GIT_SSH_COMMAND="ssh -o StrictHostKeyChecking=no" git push origin master
```
