"""Tests for hub self-registration -- role column, stale-sweep exclusion."""

from __future__ import annotations

from datetime import datetime, timezone, timedelta

import pytest

from soul_mesh.db import MeshDB
from soul_mesh.hub import Hub


@pytest.fixture
async def db():
    db = MeshDB(":memory:")
    await db.ensure_tables()
    return db


@pytest.fixture
async def hub(db):
    return Hub(db)


def _make_registration(
    node_id: str = "hub-1",
    name: str = "titan-pi",
    role: str = "hub",
    **kwargs,
) -> dict:
    defaults = {
        "node_id": node_id,
        "name": name,
        "host": "192.168.0.116",
        "port": 8340,
        "role": role,
        "platform": "linux",
        "arch": "aarch64",
        "cpu": {"cores": 4, "usage_percent": 10.0, "load_avg_1m": 0.5},
        "memory": {"total_mb": 16384, "available_mb": 8192, "used_percent": 50.0},
        "storage": {"mounts": [{"path": "/", "total_gb": 256, "free_gb": 100}]},
    }
    defaults.update(kwargs)
    return defaults


class TestHubSelfRegistration:
    """Hub registers itself with role='hub'."""

    async def test_hub_role_stored(self, hub, db):
        await hub.register_node(_make_registration())
        row = await db.fetch_one("SELECT role FROM nodes WHERE id = ?", ("hub-1",))
        assert row["role"] == "hub"

    async def test_hub_appears_in_list_nodes(self, hub):
        await hub.register_node(_make_registration())
        nodes = await hub.list_nodes()
        assert len(nodes) == 1
        assert nodes[0]["role"] == "hub"

    async def test_hub_status_online(self, hub, db):
        await hub.register_node(_make_registration())
        row = await db.fetch_one("SELECT status FROM nodes WHERE id = ?", ("hub-1",))
        assert row["status"] == "online"

    async def test_agent_defaults_to_agent_role(self, hub, db):
        await hub.register_node(_make_registration(node_id="agent-1", name="pc", role="agent"))
        row = await db.fetch_one("SELECT role FROM nodes WHERE id = ?", ("agent-1",))
        assert row["role"] == "agent"

    async def test_no_role_defaults_to_agent(self, hub, db):
        data = _make_registration(node_id="agent-2", name="pc2")
        del data["role"]
        await hub.register_node(data)
        row = await db.fetch_one("SELECT role FROM nodes WHERE id = ?", ("agent-2",))
        assert row["role"] == "agent"

    async def test_hub_and_agents_coexist(self, hub):
        await hub.register_node(_make_registration(node_id="hub-1", name="pi"))
        await hub.register_node(_make_registration(node_id="agent-1", name="pc", role="agent"))
        nodes = await hub.list_nodes()
        assert len(nodes) == 2
        roles = {n["id"]: n["role"] for n in nodes}
        assert roles["hub-1"] == "hub"
        assert roles["agent-1"] == "agent"


class TestStaleSweepSkipsHub:
    """mark_stale_nodes should never mark the hub as stale."""

    async def test_hub_not_marked_stale(self, hub, db):
        await hub.register_node(_make_registration(node_id="hub-1"))
        # Set heartbeat far in the past
        old_time = (datetime.now(timezone.utc) - timedelta(seconds=120)).strftime(
            "%Y-%m-%dT%H:%M:%SZ"
        )
        await db.execute(
            "UPDATE nodes SET last_heartbeat = ? WHERE id = ?", (old_time, "hub-1")
        )
        stale_ids = await hub.mark_stale_nodes(timeout_seconds=30)
        assert "hub-1" not in stale_ids
        row = await db.fetch_one("SELECT status FROM nodes WHERE id = ?", ("hub-1",))
        assert row["status"] == "online"

    async def test_agent_still_marked_stale(self, hub, db):
        await hub.register_node(_make_registration(node_id="hub-1"))
        await hub.register_node(_make_registration(node_id="agent-1", name="pc", role="agent"))
        old_time = (datetime.now(timezone.utc) - timedelta(seconds=120)).strftime(
            "%Y-%m-%dT%H:%M:%SZ"
        )
        await db.execute(
            "UPDATE nodes SET last_heartbeat = ? WHERE id = ?", (old_time, "agent-1")
        )
        stale_ids = await hub.mark_stale_nodes(timeout_seconds=30)
        assert "agent-1" in stale_ids
        assert "hub-1" not in stale_ids
