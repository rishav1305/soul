# Soul Courier Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace fragile per-agent bash sidecars with a single reliable Python message delivery daemon.

**Architecture:** Single Python daemon (`soul-courier.py`) watches all agent inboxes via watchdog, injects messages into tmux panes with verification, manages queues with exponential backoff, and runs as a systemd user service. Replaces 10 bash sidecars + soul-bridge.py.

**Tech Stack:** Python 3.12, watchdog 6.0.0, systemd user services, tmux

**Spec:** `docs/superpowers/specs/2026-03-23-soul-courier-design.md`

**Git strategy:** `~/.claude/scripts/` is not a git repo. All new courier files live there at runtime, but for version control, commit them into the `soul-v2` repo under `scripts/soul-courier/` with a symlink from `~/.claude/scripts/soul_courier` → `~/soul-v2/scripts/soul-courier/`. The systemd unit is tracked in soul-v2 under `deploy/`. All commit steps below assume CWD is `~/soul-v2`.

---

### Task 1: MessageQueue — In-memory deque with disk persistence

**Files:**
- Create: `~/.claude/scripts/soul_courier/queue.py`
- Test: `~/.claude/scripts/soul_courier/test_queue.py`

- [ ] **Step 1: Create package directory**

```bash
mkdir -p ~/.claude/scripts/soul_courier
touch ~/.claude/scripts/soul_courier/__init__.py
```

- [ ] **Step 2: Write failing tests for MessageQueue**

```python
# ~/.claude/scripts/soul_courier/test_queue.py
import json
import tempfile
from pathlib import Path
from soul_courier.queue import MessageQueue

def test_add_and_pop():
    with tempfile.TemporaryDirectory() as td:
        q = MessageQueue(Path(td))
        msg = Path(td) / "msg-123.json"
        msg.write_text('{"content":"hello"}')
        q.add("fury", msg)
        result = q.pop("fury")
        assert result == msg
        assert q.pop("fury") is None

def test_pop_empty_returns_none():
    with tempfile.TemporaryDirectory() as td:
        q = MessageQueue(Path(td))
        assert q.pop("fury") is None

def test_fifo_ordering():
    with tempfile.TemporaryDirectory() as td:
        q = MessageQueue(Path(td))
        msgs = []
        for i in range(3):
            m = Path(td) / f"msg-{i}.json"
            m.write_text(f'{{"i":{i}}}')
            msgs.append(m)
            q.add("fury", m)
        for i in range(3):
            assert q.pop("fury") == msgs[i]

def test_flush_and_reload():
    with tempfile.TemporaryDirectory() as td:
        q = MessageQueue(Path(td))
        msg = Path(td) / "msg-456.json"
        msg.write_text('{"content":"persist"}')
        q.add("fury", msg)
        q.flush()
        # Verify queue file written
        qf = Path(td) / "fury.json"
        assert qf.exists()
        data = json.loads(qf.read_text())
        assert len(data) == 1
        # Reload from disk
        q2 = MessageQueue(Path(td))
        q2.load()
        result = q2.pop("fury")
        assert result == msg

def test_flush_skips_missing_files():
    with tempfile.TemporaryDirectory() as td:
        q = MessageQueue(Path(td))
        ghost = Path(td) / "msg-ghost.json"  # never created
        q.add("fury", ghost)
        q.flush()
        q2 = MessageQueue(Path(td))
        q2.load()
        # Ghost file should be dropped during load validation
        assert q2.pop("fury") is None

def test_has_messages():
    with tempfile.TemporaryDirectory() as td:
        q = MessageQueue(Path(td))
        assert not q.has_messages("fury")
        msg = Path(td) / "msg-789.json"
        msg.write_text('{}')
        q.add("fury", msg)
        assert q.has_messages("fury")

def test_agents_with_messages():
    with tempfile.TemporaryDirectory() as td:
        q = MessageQueue(Path(td))
        m1 = Path(td) / "msg-1.json"; m1.write_text('{}')
        m2 = Path(td) / "msg-2.json"; m2.write_text('{}')
        q.add("fury", m1)
        q.add("hawkeye", m2)
        agents = q.agents_with_messages()
        assert set(agents) == {"fury", "hawkeye"}

def test_corrupt_queue_file_resets():
    with tempfile.TemporaryDirectory() as td:
        qf = Path(td) / "fury.json"
        qf.write_text("NOT VALID JSON {{{{")
        q = MessageQueue(Path(td))
        q.load()
        assert q.pop("fury") is None
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd ~/.claude/scripts && python3 -m pytest soul_courier/test_queue.py -v`
Expected: FAIL (module not found)

- [ ] **Step 4: Implement MessageQueue**

```python
# ~/.claude/scripts/soul_courier/queue.py
"""Per-agent message queue with in-memory deque and disk persistence."""
import json
import logging
import tempfile
from collections import defaultdict, deque
from pathlib import Path
from typing import Optional

log = logging.getLogger("soul-courier.queue")


class MessageQueue:
    def __init__(self, queue_dir: Path):
        self._dir = queue_dir
        self._dir.mkdir(parents=True, exist_ok=True)
        self._queues: dict[str, deque[Path]] = defaultdict(deque)

    def add(self, agent: str, msg_file: Path) -> None:
        self._queues[agent].append(msg_file)

    def pop(self, agent: str) -> Optional[Path]:
        q = self._queues.get(agent)
        if q:
            return q.popleft()
        return None

    def has_messages(self, agent: str) -> bool:
        q = self._queues.get(agent)
        return bool(q)

    def agents_with_messages(self) -> list[str]:
        return [a for a, q in self._queues.items() if q]

    def remove(self, agent: str, msg_file: Path) -> None:
        """Remove a specific file from agent's queue."""
        q = self._queues.get(agent)
        if q:
            try:
                q.remove(msg_file)
            except ValueError:
                pass

    def peek_thread_batch(self, agent: str, min_count: int = 3):
        """Check if agent has min_count+ discussion msgs from same thread.
        Returns (thread_id, [files]) or None."""
        q = self._queues.get(agent)
        if not q or len(q) < min_count:
            return None
        threads: dict[str, list[Path]] = {}
        for f in q:
            if not f.exists():
                continue
            try:
                data = json.loads(f.read_text())
                tid = data.get("thread_id", "")
                if tid and data.get("type") == "group-discussion":
                    threads.setdefault(tid, []).append(f)
            except (json.JSONDecodeError, OSError):
                continue
        for tid, files in threads.items():
            if len(files) >= min_count:
                return (tid, files)
        return None

    def flush(self) -> None:
        """Write all queues to disk as JSON arrays of file paths."""
        for agent, q in self._queues.items():
            qf = self._dir / f"{agent}.json"
            data = [str(p) for p in q]
            tmp = None
            try:
                tmp = tempfile.NamedTemporaryFile(
                    mode="w", dir=self._dir, suffix=".tmp", delete=False
                )
                json.dump(data, tmp)
                tmp.close()
                Path(tmp.name).rename(qf)
            except Exception:
                log.exception("Failed to flush queue for %s", agent)
                if tmp:
                    Path(tmp.name).unlink(missing_ok=True)

    def load(self) -> None:
        """Load queues from disk, validating file paths exist."""
        for qf in self._dir.glob("*.json"):
            agent = qf.stem
            try:
                data = json.loads(qf.read_text())
                if not isinstance(data, list):
                    raise ValueError("not a list")
                valid = [Path(p) for p in data if Path(p).exists()]
                if valid:
                    self._queues[agent] = deque(valid)
            except (json.JSONDecodeError, ValueError):
                log.warning("Corrupt queue file %s — resetting", qf)
                qf.write_text("[]")
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd ~/.claude/scripts && python3 -m pytest soul_courier/test_queue.py -v`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
cd ~/.claude/scripts && git add soul_courier/
git commit -m "feat(courier): add MessageQueue with disk persistence"
```

---

### Task 2: PaneManager — State detection, injection, verification

**Files:**
- Create: `~/.claude/scripts/soul_courier/pane.py`
- Test: `~/.claude/scripts/soul_courier/test_pane.py`

- [ ] **Step 1: Write failing tests for PaneManager**

```python
# ~/.claude/scripts/soul_courier/test_pane.py
import time
from unittest.mock import patch, MagicMock
from soul_courier.pane import PaneManager

def _make_pm(panes=None):
    return PaneManager(panes or {"fury": "%10", "hawkeye": "%11"})

def test_detect_state_dead_no_pane():
    pm = _make_pm({"fury": "%10"})
    assert pm.detect_state("nonexistent") == "dead"

def test_detect_state_dead_capture_fails():
    pm = _make_pm()
    with patch.object(pm, "_tmux_capture", return_value=None):
        assert pm.detect_state("fury") == "dead"

def test_detect_state_crashed():
    pm = _make_pm()
    content = "some output\nrishav@titan-pi:~$ \n"
    with patch.object(pm, "_tmux_capture", return_value=content):
        assert pm.detect_state("fury") == "crashed"

def test_detect_state_idle_prompt():
    pm = _make_pm()
    content = "some output\n❯ \nstatus bar line"
    with patch.object(pm, "_tmux_capture", return_value=content):
        assert pm.detect_state("fury") == "idle"

def test_detect_state_idle_ascii_prompt():
    pm = _make_pm()
    content = "some output\n> \nstatus bar"
    with patch.object(pm, "_tmux_capture", return_value=content):
        assert pm.detect_state("fury") == "idle"

def test_detect_state_crunched():
    pm = _make_pm()
    content = "Crunched previous messages\n❯ \nstatus"
    with patch.object(pm, "_tmux_capture", return_value=content):
        assert pm.detect_state("fury") == "crunched"

def test_detect_state_cache_hit():
    pm = _make_pm()
    content = "❯ \nstatus"
    with patch.object(pm, "_tmux_capture", return_value=content) as mock_cap:
        assert pm.detect_state("fury") == "idle"
        # Second call within 3s should use cache
        assert pm.detect_state("fury") == "idle"
        assert mock_cap.call_count == 1  # Only called once

def test_detect_state_cache_expired():
    pm = _make_pm()
    content = "❯ \nstatus"
    with patch.object(pm, "_tmux_capture", return_value=content) as mock_cap:
        assert pm.detect_state("fury") == "idle"
        # Expire cache
        pm._state_cache["fury"] = ("idle", time.monotonic() - 5.0)
        assert pm.detect_state("fury") == "idle"
        assert mock_cap.call_count == 2

def test_inject_uses_named_buffer():
    pm = _make_pm()
    with patch("subprocess.run") as mock_run:
        mock_run.return_value = MagicMock(returncode=0)
        result = pm.inject("fury", "hello world")
        assert result is True
        # Check named buffer was used
        load_call = mock_run.call_args_list[0]
        assert "-b" in load_call.args[0]
        assert "courier-fury" in load_call.args[0]

def test_verify_injection_success():
    pm = _make_pm()
    with patch.object(pm, "_tmux_capture", return_value="[INBOX] From: team-lead | blah"):
        assert pm.verify_injection("fury", "From: team-lead") is True

def test_verify_injection_failure():
    pm = _make_pm()
    with patch.object(pm, "_tmux_capture", return_value="some unrelated output"):
        assert pm.verify_injection("fury", "From: team-lead") is False

def test_update_panes():
    pm = _make_pm()
    pm.update_panes({"fury": "%20", "loki": "%21"})
    assert pm.panes["fury"] == "%20"
    assert pm.panes["loki"] == "%21"
    assert "hawkeye" not in pm.panes

def test_mark_dead():
    pm = _make_pm()
    pm.mark_dead("fury")
    assert pm.detect_state("fury") == "dead"
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ~/.claude/scripts && python3 -m pytest soul_courier/test_pane.py -v`
Expected: FAIL (module not found)

- [ ] **Step 3: Implement PaneManager**

```python
# ~/.claude/scripts/soul_courier/pane.py
"""Tmux pane state detection, message injection, and verification."""
import logging
import os
import re
import subprocess
import tempfile
import time
import threading
from typing import Optional

log = logging.getLogger("soul-courier.pane")

CRASHED_RE = re.compile(r'^\$\s*$|^[a-z]+@[a-zA-Z0-9-]+[^>]*\$\s*$', re.M)
IDLE_RE = re.compile(r'[❯>]\s*$')


class PaneManager:
    def __init__(self, panes: dict[str, str]):
        self.panes: dict[str, str] = dict(panes)
        self.locks: dict[str, threading.Lock] = {a: threading.Lock() for a in panes}
        self._state_cache: dict[str, tuple[str, float]] = {}
        self._dead_agents: set[str] = set()

    def detect_state(self, agent: str) -> str:
        """Detect pane state: idle, crunched, busy, crashed, dead.
        Uses 3s TTL cache to avoid redundant detection during bursts."""
        if agent in self._dead_agents:
            return "dead"

        cached = self._state_cache.get(agent)
        if cached and time.monotonic() - cached[1] < 3.0:
            return cached[0]

        pane_id = self.panes.get(agent)
        if not pane_id:
            return "dead"

        content = self._tmux_capture(pane_id, lines=15)
        if content is None:
            return "dead"

        # Crashed: bash prompt detected
        if CRASHED_RE.search(content):
            state = "crashed"
            self._state_cache[agent] = (state, time.monotonic())
            return state

        # Idle: ❯ or > at end of any non-empty line
        non_empty = [l for l in content.splitlines() if l.strip()]
        for line in non_empty:
            if IDLE_RE.search(line):
                state = "crunched" if "Crunched" in content else "idle"
                self._state_cache[agent] = (state, time.monotonic())
                return state

        if "Crunched" in content:
            state = "crunched"
            self._state_cache[agent] = (state, time.monotonic())
            return state

        # Ambiguous: compare snapshots 1.5s apart
        snapshot1 = content
        time.sleep(1.5)
        snapshot2 = self._tmux_capture(pane_id, lines=15)

        if snapshot2 and any(IDLE_RE.search(l) for l in snapshot2.splitlines() if l.strip()):
            state = "crunched" if "Crunched" in snapshot2 else "idle"
        elif snapshot1 == snapshot2:
            state = "idle"
        else:
            state = "busy"

        self._state_cache[agent] = (state, time.monotonic())
        return state

    def inject(self, agent: str, text: str) -> bool:
        """Inject text via tmux named buffer. Returns True on success."""
        pane_id = self.panes.get(agent)
        if not pane_id:
            return False
        buf_name = f"courier-{agent}"
        tmp_path = None
        try:
            fd, tmp_path = tempfile.mkstemp(suffix=".txt")
            with os.fdopen(fd, "w") as f:
                f.write(text)
            subprocess.run(
                ["tmux", "load-buffer", "-b", buf_name, tmp_path], check=True,
                capture_output=True, timeout=5
            )
            subprocess.run(
                ["tmux", "paste-buffer", "-b", buf_name, "-t", pane_id, "-d"], check=True,
                capture_output=True, timeout=5
            )
            time.sleep(0.3)
            subprocess.run(
                ["tmux", "send-keys", "-t", pane_id, "Enter"], check=True,
                capture_output=True, timeout=5
            )
            return True
        except (subprocess.CalledProcessError, subprocess.TimeoutExpired) as e:
            log.warning("Inject failed for %s: %s", agent, e)
            return False
        finally:
            if tmp_path:
                try:
                    os.unlink(tmp_path)
                except OSError:
                    pass

    def verify_injection(self, agent: str, fragment: str, wait: float = 2.0) -> bool:
        """Wait, then check if fragment appears in pane content."""
        time.sleep(wait)
        pane_id = self.panes.get(agent)
        if not pane_id:
            return False
        content = self._tmux_capture(pane_id, lines=30)
        return content is not None and fragment in content

    def update_panes(self, panes: dict[str, str]) -> None:
        """Update pane mapping. Preserves existing locks to avoid racing in-flight deliveries."""
        self.panes = dict(panes)
        # Only add locks for new agents, keep existing ones
        for a in panes:
            if a not in self.locks:
                self.locks[a] = threading.Lock()
        # Remove locks for agents no longer present
        for a in list(self.locks):
            if a not in panes:
                del self.locks[a]
        self._state_cache.clear()
        self._dead_agents.clear()

    def mark_dead(self, agent: str) -> None:
        self._dead_agents.add(agent)
        self.panes.pop(agent, None)

    def invalidate_cache(self, agent: str) -> None:
        self._state_cache.pop(agent, None)

    def _tmux_capture(self, pane_id: str, lines: int = 15) -> Optional[str]:
        try:
            result = subprocess.run(
                ["tmux", "capture-pane", "-p", "-t", pane_id, "-S", f"-{lines}"],
                capture_output=True, text=True, timeout=5
            )
            if result.returncode != 0:
                return None
            return result.stdout
        except (subprocess.TimeoutExpired, FileNotFoundError):
            return None
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd ~/.claude/scripts && python3 -m pytest soul_courier/test_pane.py -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
cd ~/.claude/scripts && git add soul_courier/pane.py soul_courier/test_pane.py
git commit -m "feat(courier): add PaneManager with state detection and injection"
```

---

### Task 3: MessageFormatter — Format messages for pane injection

**Files:**
- Create: `~/.claude/scripts/soul_courier/formatter.py`
- Test: `~/.claude/scripts/soul_courier/test_formatter.py`

- [ ] **Step 1: Write failing tests**

```python
# ~/.claude/scripts/soul_courier/test_formatter.py
import json
import tempfile
from pathlib import Path
from soul_courier.formatter import MessageFormatter

def _write_msg(td, msg_type="message", **kwargs):
    data = {"type": msg_type, "from": "team-lead", "to": "fury",
            "content": "Hello fury", **kwargs}
    f = Path(td) / "msg-123.json"
    f.write_text(json.dumps(data))
    return f

def test_format_direct():
    with tempfile.TemporaryDirectory() as td:
        f = _write_msg(td)
        text = MessageFormatter.format(f, "fury")
        assert "[INBOX] From: team-lead" in text
        assert "Hello fury" in text
        assert "clawteam inbox send" in text

def test_format_broadcast():
    with tempfile.TemporaryDirectory() as td:
        f = _write_msg(td, msg_type="broadcast")
        text = MessageFormatter.format(f, "fury")
        assert "[BROADCAST]" in text
        assert "team-lead" in text

def test_format_ceo_minimal():
    with tempfile.TemporaryDirectory() as td:
        f = _write_msg(td, **{"from": "fury"})
        text = MessageFormatter.format(f, "team-lead")
        assert "[fury]" in text
        assert "Hello fury" in text
        # CEO format should NOT contain clawteam instructions
        assert "clawteam" not in text

def test_format_p1_interrupt():
    with tempfile.TemporaryDirectory() as td:
        f = _write_msg(td, key="urgent")
        text = MessageFormatter.format_p1(f, "fury")
        assert "[P1 INTERRUPT" in text
        assert "STEP 1" in text
        assert "STEP 2" in text
        assert "STEP 3" in text

def test_format_discussion():
    with tempfile.TemporaryDirectory() as td:
        f = _write_msg(td, msg_type="group-discussion", thread_id="thread-1")
        text = MessageFormatter.format(f, "fury")
        assert "[DISCUSSION: thread-1]" in text

def test_format_status():
    with tempfile.TemporaryDirectory() as td:
        data = {"type": "status", "from": "sidecar", "action": "crashed",
                "agent": "pepper", "content": "Agent pepper crashed"}
        f = Path(td) / "msg-status.json"
        f.write_text(json.dumps(data))
        text = MessageFormatter.format(f, "team-lead")
        assert "[sidecar]" in text

def test_format_batch():
    with tempfile.TemporaryDirectory() as td:
        files = []
        for i in range(3):
            data = {"type": "group-discussion", "from": f"agent-{i}",
                    "content": f"Message {i}", "thread_id": "t1"}
            f = Path(td) / f"msg-{i}.json"
            f.write_text(json.dumps(data))
            files.append(f)
        text = MessageFormatter.format_batch("t1", files, "fury")
        assert "3 new messages" in text
        assert "agent-0" in text
        assert "agent-2" in text
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ~/.claude/scripts && python3 -m pytest soul_courier/test_formatter.py -v`
Expected: FAIL

- [ ] **Step 3: Implement MessageFormatter**

```python
# ~/.claude/scripts/soul_courier/formatter.py
"""Format messages for tmux pane injection. Matches existing sidecar output formats."""
import json
import logging
import time
from pathlib import Path

log = logging.getLogger("soul-courier.formatter")


class MessageFormatter:
    @staticmethod
    def _read(msg_file: Path) -> dict:
        try:
            return json.loads(msg_file.read_text())
        except (json.JSONDecodeError, OSError):
            log.warning("Cannot read message %s", msg_file)
            return {}

    @classmethod
    def format(cls, msg_file: Path, agent: str, is_crunched: bool = False) -> str:
        data = cls._read(msg_file)
        if not data:
            return ""

        msg_type = data.get("type", "message")
        from_user = data.get("from", "unknown")
        content = data.get("content", "")
        action = data.get("action", "")
        thread_id = data.get("thread_id", "")

        # CEO gets minimal format
        if agent == "team-lead":
            if action and action != "null":
                return f"[{from_user}] {action}: {content}"
            return f"[{from_user}] {content}"

        if msg_type == "broadcast":
            return (
                f"[BROADCAST] From: {from_user}\n"
                f"---\n{content}\n---\n"
                f"Respond to CEO inbox via: clawteam inbox send soul-team "
                f"team-lead \"your response\" --from {agent}"
            )

        if msg_type == "group-discussion":
            ts = int(time.time())
            msg_count = data.get("message_count", "?")
            base = (
                f"[DISCUSSION: {thread_id}] From: {from_user} (message {msg_count})\n"
                f"---\n{content}\n---\n"
                f"Respond by writing to discussions/{thread_id}/ "
                f"with filename: {ts}-{agent}.json\n"
                f"Keep it under 200 words. Reference peers by name."
            )
            if is_crunched and thread_id:
                summary = cls._build_thread_summary(thread_id)
                if summary:
                    base = f"Thread summary (context was compressed):\n{summary}\n{base}"
            return base

        if msg_type == "status":
            if action and action != "null":
                return f"[{from_user}] {action}: {content}"
            return f"[{from_user}] {content}"

        # Default: direct message
        return (
            f"[INBOX] From: {from_user} | Type: {msg_type}\n"
            f"---\n{content}\n---\n"
            f"Respond by writing to {from_user}'s inbox via: "
            f"clawteam inbox send soul-team {from_user} "
            f"\"your response\" --from {agent}"
        )

    @classmethod
    def format_p1(cls, msg_file: Path, agent: str) -> str:
        data = cls._read(msg_file)
        from_user = data.get("from", "unknown")
        content = data.get("content", "")
        if from_user in ("unknown", "agent", ""):
            from_user = "team-lead"

        return (
            f"[P1 INTERRUPT from {from_user}]\n\n"
            f"STEP 1: Save your interrupted state to memory NOW.\n"
            f"Write a project memory titled \"interrupted-state\" containing:\n"
            f"- What task/routine step you were executing\n"
            f"- What you had completed so far\n"
            f"- What steps remain\n\n"
            f"STEP 2: Handle this message:\n"
            f"---\n{content}\n---\n"
            f"Respond via: clawteam inbox send soul-team {from_user} "
            f"\"your response\" --from {agent}\n\n"
            f"STEP 3: After handling, check your memory for \"interrupted-state\". "
            f"If found:\n"
            f"- Read it to recall where you left off\n"
            f"- Delete the memory (it is consumed)\n"
            f"- Continue from where you stopped"
        )

    @classmethod
    def format_batch(cls, thread_id: str, files: list[Path], agent: str) -> str:
        ts = int(time.time())
        count = len(files)
        lines = [f"[DISCUSSION: {thread_id}] {count} new messages since your last response:"]
        for f in files:
            data = cls._read(f)
            from_user = data.get("from", "unknown")
            excerpt = data.get("content", "")[:120]
            lines.append(f'- {from_user}: "{excerpt}"')
        lines.append("---")
        lines.append(
            f"Respond to the thread or say \"acknowledged\" if nothing to add.\n"
            f"Write to discussions/{thread_id}/{ts}-{agent}.json"
        )
        return "\n".join(lines)

    @staticmethod
    def _build_thread_summary(thread_id: str) -> str:
        disc_dir = Path.home() / ".clawteam/teams/soul-team/discussions" / thread_id
        if not disc_dir.is_dir():
            return ""
        lines = []
        for f in sorted(disc_dir.glob("*.json")):
            if f.name == "state.json":
                continue
            try:
                data = json.loads(f.read_text())
                from_user = data.get("from", "unknown")
                excerpt = data.get("content", "")[:100]
                lines.append(f'- {from_user}: "{excerpt}"')
            except (json.JSONDecodeError, OSError):
                continue
        return "\n".join(lines)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd ~/.claude/scripts && python3 -m pytest soul_courier/test_formatter.py -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
cd ~/.claude/scripts && git add soul_courier/formatter.py soul_courier/test_formatter.py
git commit -m "feat(courier): add MessageFormatter with all message types"
```

---

### Task 4: InboxWatcher — Watchdog filesystem event handler

**Files:**
- Create: `~/.claude/scripts/soul_courier/watcher.py`
- Test: `~/.claude/scripts/soul_courier/test_watcher.py`

- [ ] **Step 1: Write failing tests**

```python
# ~/.claude/scripts/soul_courier/test_watcher.py
import tempfile
import time
from pathlib import Path
from unittest.mock import MagicMock
from watchdog.events import FileMovedEvent
from soul_courier.watcher import InboxWatcher

def test_parse_agent_from_path():
    w = InboxWatcher(callback=MagicMock())
    assert w._parse_agent("/inboxes/fury_fury/msg-123.json") == "fury"
    assert w._parse_agent("/inboxes/team-lead_team-lead/msg-456.json") == "team-lead"
    assert w._parse_agent("/inboxes/fury_fury/archive/msg-123.json") is None
    assert w._parse_agent("/inboxes/fury_fury/.tmp-123.json") is None
    assert w._parse_agent("/inboxes/fury_fury/not-a-msg.txt") is None

def test_on_moved_dispatches():
    cb = MagicMock()
    w = InboxWatcher(callback=cb)
    event = FileMovedEvent(
        src_path="/tmp/.tmp-123.json",
        dest_path="/home/rishav/.clawteam/teams/soul-team/inboxes/fury_fury/msg-123.json"
    )
    w.on_moved(event)
    cb.assert_called_once()
    args = cb.call_args
    assert args[0][0] == "fury"
    assert "msg-123.json" in str(args[0][1])

def test_on_moved_ignores_archive():
    cb = MagicMock()
    w = InboxWatcher(callback=cb)
    event = FileMovedEvent(
        src_path="/tmp/.tmp-123.json",
        dest_path="/inboxes/fury_fury/archive/msg-123.json"
    )
    w.on_moved(event)
    cb.assert_not_called()

def test_on_moved_ignores_non_json():
    cb = MagicMock()
    w = InboxWatcher(callback=cb)
    event = FileMovedEvent(
        src_path="/tmp/file.txt",
        dest_path="/inboxes/fury_fury/file.txt"
    )
    w.on_moved(event)
    cb.assert_not_called()
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ~/.claude/scripts && python3 -m pytest soul_courier/test_watcher.py -v`
Expected: FAIL

- [ ] **Step 3: Implement InboxWatcher**

```python
# ~/.claude/scripts/soul_courier/watcher.py
"""Watchdog event handler for agent inbox directories."""
import logging
import re
from pathlib import Path
from typing import Callable, Optional

from watchdog.events import FileSystemEventHandler, FileMovedEvent

log = logging.getLogger("soul-courier.watcher")

# Match: inboxes/{agent}_{agent}/msg-*.json (but not archive/ or .tmp-)
INBOX_RE = re.compile(r'/inboxes/([a-z][\w-]*)_\1/(msg-[^/]+\.json)$')


class InboxWatcher(FileSystemEventHandler):
    """Watches inbox directories for atomic renames (moved_to events)."""

    def __init__(self, callback: Callable[[str, Path], None]):
        super().__init__()
        self._callback = callback

    def on_moved(self, event: FileMovedEvent) -> None:
        if event.is_directory:
            return
        dest = event.dest_path
        agent = self._parse_agent(dest)
        if agent:
            log.info("New message for %s: %s", agent, Path(dest).name)
            self._callback(agent, Path(dest))

    def _parse_agent(self, path: str) -> Optional[str]:
        """Extract agent name from inbox path. Returns None if path should be ignored."""
        p = Path(path)
        # Ignore non-JSON
        if p.suffix != ".json":
            return None
        # Ignore .tmp files
        if p.name.startswith(".tmp"):
            return None
        # Ignore archive subdirectory
        if "archive" in p.parts:
            return None
        # Match inbox pattern
        m = INBOX_RE.search(path)
        if m:
            return m.group(1)
        return None
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd ~/.claude/scripts && python3 -m pytest soul_courier/test_watcher.py -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
cd ~/.claude/scripts && git add soul_courier/watcher.py soul_courier/test_watcher.py
git commit -m "feat(courier): add InboxWatcher with watchdog event handling"
```

---

### Task 5: CourierDaemon — Main daemon with all threads

**Files:**
- Create: `~/.claude/scripts/soul_courier/daemon.py`
- Create: `~/.claude/scripts/soul-courier.py` (entry point)
- Test: `~/.claude/scripts/soul_courier/test_daemon.py`

- [ ] **Step 1: Write failing tests for key daemon behaviors**

```python
# ~/.claude/scripts/soul_courier/test_daemon.py
import json
import tempfile
import time
from pathlib import Path
from unittest.mock import patch, MagicMock
from soul_courier.daemon import CourierDaemon

def _setup_dirs(td):
    """Create minimal directory structure for daemon."""
    base = Path(td)
    inboxes = base / "inboxes" / "fury_fury"
    inboxes.mkdir(parents=True)
    (inboxes / "archive").mkdir()
    queue = base / "queue"
    queue.mkdir()
    sidecar = base / "sidecar"
    sidecar.mkdir()
    panes = base / "panes.json"
    panes.write_text(json.dumps({"fury": "%10"}))
    return base, panes

def test_load_panes():
    with tempfile.TemporaryDirectory() as td:
        base, panes = _setup_dirs(td)
        daemon = CourierDaemon(team_dir=base, panes_file=panes, dry_run=True)
        assert daemon.pane_mgr.panes == {"fury": "%10"}

def test_is_seen_and_mark_seen():
    with tempfile.TemporaryDirectory() as td:
        base, panes = _setup_dirs(td)
        daemon = CourierDaemon(team_dir=base, panes_file=panes, dry_run=True)
        msg = Path(td) / "msg-test-123.json"
        assert not daemon._is_seen(msg)
        daemon._mark_seen(msg)
        assert daemon._is_seen(msg)

def test_catchup_processes_unseen():
    with tempfile.TemporaryDirectory() as td:
        base, panes = _setup_dirs(td)
        inbox = base / "inboxes" / "fury_fury"
        msg = inbox / "msg-catchup-1.json"
        msg.write_text(json.dumps({"type": "message", "from": "ceo", "content": "test"}))
        daemon = CourierDaemon(team_dir=base, panes_file=panes, dry_run=True)
        with patch.object(daemon, "_deliver") as mock_deliver:
            daemon.catchup()
            mock_deliver.assert_called_once_with("fury", msg)

def test_catchup_skips_seen():
    with tempfile.TemporaryDirectory() as td:
        base, panes = _setup_dirs(td)
        inbox = base / "inboxes" / "fury_fury"
        msg = inbox / "msg-seen-1.json"
        msg.write_text(json.dumps({"type": "message", "from": "ceo", "content": "old"}))
        daemon = CourierDaemon(team_dir=base, panes_file=panes, dry_run=True)
        daemon._mark_seen(msg)
        with patch.object(daemon, "_deliver") as mock_deliver:
            daemon.catchup()
            mock_deliver.assert_not_called()
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ~/.claude/scripts && python3 -m pytest soul_courier/test_daemon.py -v`
Expected: FAIL

- [ ] **Step 3: Implement CourierDaemon**

```python
# ~/.claude/scripts/soul_courier/daemon.py
"""Soul Courier — Single daemon managing message delivery for all agents."""
import json
import logging
import os
import signal
import subprocess
import threading
import time
from pathlib import Path
from typing import Optional

from watchdog.observers import Observer

from soul_courier.formatter import MessageFormatter
from soul_courier.pane import PaneManager
from soul_courier.queue import MessageQueue
from soul_courier.watcher import InboxWatcher

log = logging.getLogger("soul-courier")

TEAM_NAME = os.environ.get("SOUL_TEAM_NAME", "soul-team")
TEAM_DIR = Path.home() / ".clawteam" / "teams" / TEAM_NAME
PANES_FILE = TEAM_DIR / "panes.json"
NATIVE_INBOX_DIR = Path.home() / ".claude" / "teams" / TEAM_NAME / "inboxes"


class CourierDaemon:
    def __init__(
        self,
        team_dir: Path = TEAM_DIR,
        panes_file: Path = PANES_FILE,
        dry_run: bool = False,
    ):
        self.team_dir = team_dir
        self.panes_file = panes_file
        self.dry_run = dry_run
        self._running = False

        # Load panes
        panes = self._load_panes()
        self.pane_mgr = PaneManager(panes)
        self.queue = MessageQueue(team_dir / "queue")
        self.queue.load()

        # Seen tracking
        self._sidecar_dir = team_dir / "sidecar"
        self._sidecar_dir.mkdir(parents=True, exist_ok=True)
        self._seen: dict[str, set[str]] = {}
        self._load_seen_logs()

        # Backoff tracking
        self._backoff: dict[str, float] = {}
        self._fail_count: dict[str, int] = {}
        self._last_drain: dict[str, float] = {}

        # Observer
        self._observer: Optional[Observer] = None

    def _load_panes(self) -> dict[str, str]:
        try:
            return json.loads(self.panes_file.read_text())
        except (json.JSONDecodeError, FileNotFoundError, OSError):
            log.error("Cannot read panes.json at %s", self.panes_file)
            return {}

    def _load_seen_logs(self) -> None:
        for f in self._sidecar_dir.glob("*-seen.log"):
            agent = f.stem.replace("-seen", "")
            try:
                self._seen[agent] = set(f.read_text().splitlines())
            except OSError:
                self._seen[agent] = set()

    def _is_seen(self, agent: str, msg_file: Path) -> bool:
        msg_id = msg_file.stem
        return msg_id in self._seen.get(agent, set())

    def _mark_seen(self, msg_file: Path) -> None:
        msg_id = msg_file.stem
        # Determine agent from path
        agent = self._agent_from_path(msg_file)
        if agent not in self._seen:
            self._seen[agent] = set()
        self._seen[agent].add(msg_id)
        # Append to seen log
        seen_log = self._sidecar_dir / f"{agent}-seen.log"
        try:
            with open(seen_log, "a") as f:
                f.write(msg_id + "\n")
        except OSError:
            log.warning("Cannot write seen log for %s", agent)

    def _agent_from_path(self, msg_file: Path) -> str:
        """Extract agent name from inbox path."""
        parts = msg_file.parts
        for i, p in enumerate(parts):
            if p == "inboxes" and i + 1 < len(parts):
                dirname = parts[i + 1]
                return dirname.split("_")[0]
        return "unknown"

    def _archive(self, msg_file: Path) -> None:
        archive_dir = msg_file.parent / "archive"
        archive_dir.mkdir(exist_ok=True)
        try:
            msg_file.rename(archive_dir / msg_file.name)
        except OSError:
            log.warning("Cannot archive %s", msg_file)

    def _mirror_native(self, msg_file: Path, agent: str) -> None:
        """Mirror message to native inbox (team-lead only).
        Writes JSON array format matching existing soul-bridge convention."""
        if agent != "team-lead":
            return
        import fcntl
        native_file = NATIVE_INBOX_DIR / "team-lead.json"
        native_file.parent.mkdir(parents=True, exist_ok=True)
        try:
            raw = json.loads(msg_file.read_text())
            # Normalize to native format (bridge convention)
            entry = {
                "from": raw.get("from", "unknown"),
                "text": raw.get("content", ""),
                "timestamp": raw.get("timestamp", time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())),
                "read": False,
            }
            with open(native_file, "r+") if native_file.exists() else open(native_file, "w") as f:
                fcntl.flock(f, fcntl.LOCK_EX)
                try:
                    content = f.read()
                    msgs = json.loads(content) if content.strip() else []
                    if not isinstance(msgs, list):
                        msgs = []
                    msgs.append(entry)
                    f.seek(0)
                    f.truncate()
                    json.dump(msgs, f, indent=2)
                finally:
                    fcntl.flock(f, fcntl.LOCK_UN)
        except (json.JSONDecodeError, OSError):
            log.warning("Cannot mirror to native inbox: %s", msg_file.name)

    def _notify_ceo(self, agent: str, status: str) -> None:
        """Write crash/dead notification to CEO inbox."""
        ceo_inbox = self.team_dir / "inboxes" / "team-lead_team-lead"
        ceo_inbox.mkdir(parents=True, exist_ok=True)
        ts = int(time.time())
        data = {
            "type": "status", "from": "courier", "to": "team-lead",
            "action": status, "agent": agent,
            "content": f"Agent {agent} pane is {status}.",
            "ts": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        }
        # Atomic write
        import tempfile as tf
        tmp = ceo_inbox / f".tmp-{ts}.json"
        target = ceo_inbox / f"msg-{ts}-courier-{agent}.json"
        tmp.write_text(json.dumps(data))
        tmp.rename(target)

    def _deliver(self, agent: str, msg_file: Path) -> bool:
        """Deliver a single message to an agent. Core delivery logic."""
        if not msg_file.exists():
            return False

        state = self.pane_mgr.detect_state(agent)
        try:
            msg_data = json.loads(msg_file.read_text())
        except (json.JSONDecodeError, OSError):
            log.warning("Cannot read message %s", msg_file)
            return False
        priority = msg_data.get("key", "normal")

        # P1 interrupt
        if priority == "urgent" and state == "busy":
            state = self._p1_interrupt(agent)

        lock = self.pane_mgr.locks.get(agent)
        if not lock:
            self.queue.add(agent, msg_file)
            return False

        with lock:
            if state in ("idle", "crunched"):
                is_crunched = state == "crunched"
                if priority == "urgent":
                    text = MessageFormatter.format_p1(msg_file, agent)
                else:
                    text = MessageFormatter.format(msg_file, agent, is_crunched=is_crunched)

                if not self.dry_run and self.pane_mgr.inject(agent, text):
                    from_user = msg_data.get("from", "unknown")
                    fragment = f"From: {from_user}"
                    if self.pane_mgr.verify_injection(agent, fragment):
                        self._archive(msg_file)
                        self._mark_seen(msg_file)
                        self._mirror_native(msg_file, agent)
                        self._backoff.pop(agent, None)
                        self._fail_count.pop(agent, None)
                        log.info("Delivered to %s: %s", agent, msg_file.name)
                        return True

                # Failed
                self.queue.add(agent, msg_file)
                self._increment_fail(agent)
                return False

            # Not idle — queue
            self.queue.add(agent, msg_file)
            if state == "crashed":
                self._notify_ceo(agent, "crashed")
            elif state == "dead":
                self._notify_ceo(agent, "dead")
            self._mirror_native(msg_file, agent)
            return False

    def _p1_interrupt(self, agent: str) -> str:
        lock_file = self._sidecar_dir / f"{agent}-interrupt.lock"
        if lock_file.exists():
            try:
                age = time.time() - lock_file.stat().st_mtime
                if age < 30:
                    return "busy"
            except OSError:
                pass

        pane_id = self.pane_mgr.panes.get(agent)
        if not pane_id:
            return "dead"

        for _ in range(3):
            subprocess.run(["tmux", "send-keys", "-t", pane_id, "C-c"],
                           capture_output=True, timeout=5)
            time.sleep(2)
            self.pane_mgr.invalidate_cache(agent)
            state = self.pane_mgr.detect_state(agent)
            if state in ("idle", "crunched"):
                lock_file.touch()
                return state
        return "busy"

    def _increment_fail(self, agent: str) -> None:
        count = self._fail_count.get(agent, 0) + 1
        self._fail_count[agent] = count
        self._backoff[agent] = min(10 * (2 ** (count - 1)), 120)
        if count >= 5 and count % 5 == 0:
            log.warning("%d consecutive failures for %s", count, agent)
            self._notify_ceo(agent, f"delivery-failing ({count} attempts)")

    # ── Catch-up ──────────────────────────────────────────────────────────────
    def catchup(self) -> None:
        """Process any unseen messages in all inboxes."""
        inboxes_dir = self.team_dir / "inboxes"
        count = 0
        for agent_dir in sorted(inboxes_dir.iterdir()):
            if not agent_dir.is_dir() or agent_dir.name == "agent":
                continue
            agent = agent_dir.name.split("_")[0]
            for msg in sorted(agent_dir.glob("msg-*.json")):
                if not self._is_seen(agent, msg):
                    self._deliver(agent, msg)
                    count += 1
        log.info("Catch-up complete: %d messages processed", count)

    # ── Queue Drainer ─────────────────────────────────────────────────────────
    def _drain_loop(self) -> None:
        while self._running:
            for agent in self.queue.agents_with_messages():
                backoff = self._backoff.get(agent, 0)
                last = self._last_drain.get(agent, 0)
                if time.monotonic() - last < backoff:
                    continue
                self._last_drain[agent] = time.monotonic()

                # Overflow batching: check for 3+ discussion msgs from same thread
                batch = self.queue.peek_thread_batch(agent, min_count=3)
                if batch:
                    thread_id, files = batch
                    state = self.pane_mgr.detect_state(agent)
                    if state in ("idle", "crunched"):
                        text = MessageFormatter.format_batch(thread_id, files, agent)
                        with self.pane_mgr.locks.get(agent, threading.Lock()):
                            if self.pane_mgr.inject(agent, text):
                                for f in files:
                                    self.queue.remove(agent, f)
                                    self._archive(f)
                                    self._mark_seen(f)
                                continue

                msg = self.queue.pop(agent)
                if msg and msg.exists():
                    self._deliver(agent, msg)
                elif msg:
                    log.warning("Queued file missing: %s", msg)
            time.sleep(10)

    # ── Health Checker ────────────────────────────────────────────────────────
    def _health_loop(self) -> None:
        while self._running:
            time.sleep(60)
            try:
                self._run_health_check()
            except Exception:
                log.exception("Health check failed")

    def _run_health_check(self) -> None:
        # Reload panes
        new_panes = self._load_panes()
        if not new_panes:
            return

        # Get live tmux panes
        try:
            result = subprocess.run(
                ["tmux", "list-panes", "-s", "-t", "soul-team", "-F", "#{pane_id}"],
                capture_output=True, text=True, timeout=5
            )
            live_panes = set(result.stdout.strip().splitlines())
        except (subprocess.CalledProcessError, subprocess.TimeoutExpired):
            log.warning("Cannot list tmux panes")
            return

        # Mark dead agents
        for agent, pane_id in new_panes.items():
            if pane_id not in live_panes:
                log.warning("Agent %s pane %s is dead", agent, pane_id)
                self.pane_mgr.mark_dead(agent)
            elif agent in self.pane_mgr._dead_agents:
                # Revived
                self.pane_mgr._dead_agents.discard(agent)

        self.pane_mgr.update_panes(
            {a: p for a, p in new_panes.items() if p in live_panes}
        )

        # Kill orphaned legacy processes (migration period)
        subprocess.run(["pkill", "-f", "soul-sidecar"], capture_output=True)
        subprocess.run(["pkill", "-f", "soul-bridge"], capture_output=True)

        # Verify observer is alive
        if self._observer and not self._observer.is_alive():
            log.warning("Watchdog observer died — restarting")
            self._start_observer()

        # Flush queues
        self.queue.flush()

        log.info("Health check OK: %d active agents", len(self.pane_mgr.panes))

    # ── Observer ──────────────────────────────────────────────────────────────
    def _start_observer(self) -> None:
        if self._observer and self._observer.is_alive():
            self._observer.stop()
            self._observer.join(timeout=5)

        inboxes_dir = self.team_dir / "inboxes"
        handler = InboxWatcher(callback=self._on_new_message)
        self._observer = Observer()
        self._observer.schedule(handler, str(inboxes_dir), recursive=True)
        self._observer.daemon = True
        self._observer.start()
        log.info("Watchdog observer started on %s", inboxes_dir)

    def _on_new_message(self, agent: str, msg_file: Path) -> None:
        """Callback from InboxWatcher on new message detection."""
        if self._is_seen(agent, msg_file):
            return
        time.sleep(0.2)  # Brief delay for file to be fully written
        self._deliver(agent, msg_file)

    # ── Lifecycle ─────────────────────────────────────────────────────────────
    def start(self) -> None:
        log.info("Soul Courier starting (team=%s)", TEAM_NAME)
        self._running = True

        # Catch-up
        self.catchup()
        self.queue.flush()

        # Start observer
        self._start_observer()

        # Start drain thread
        drain_thread = threading.Thread(target=self._drain_loop, name="drain", daemon=True)
        drain_thread.start()

        # Start health thread
        health_thread = threading.Thread(target=self._health_loop, name="health", daemon=True)
        health_thread.start()

        # Queue auto-flush
        def flush_loop():
            while self._running:
                time.sleep(30)
                self.queue.flush()
        flush_thread = threading.Thread(target=flush_loop, name="flush", daemon=True)
        flush_thread.start()

        log.info("Soul Courier running — %d agents", len(self.pane_mgr.panes))

        # Block main thread
        try:
            while self._running:
                time.sleep(1)
        except KeyboardInterrupt:
            pass

    def stop(self) -> None:
        log.info("Soul Courier stopping...")
        self._running = False
        if self._observer:
            self._observer.stop()
            self._observer.join(timeout=5)
        self.queue.flush()
        log.info("Soul Courier stopped.")
```

- [ ] **Step 4: Create entry point script**

```python
# ~/.claude/scripts/soul-courier.py
#!/usr/bin/env python3
"""Soul Courier — Reliable message delivery daemon for soul-team."""
import logging
import signal
import sys

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(name)s] %(levelname)s %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S",
)

from soul_courier.daemon import CourierDaemon

def main():
    daemon = CourierDaemon()

    def handle_signal(signum, frame):
        daemon.stop()
        sys.exit(0)

    signal.signal(signal.SIGTERM, handle_signal)
    signal.signal(signal.SIGINT, handle_signal)

    daemon.start()

if __name__ == "__main__":
    main()
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd ~/.claude/scripts && python3 -m pytest soul_courier/test_daemon.py -v`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
cd ~/.claude/scripts && git add soul_courier/daemon.py soul-courier.py soul_courier/test_daemon.py
git commit -m "feat(courier): add CourierDaemon with all threads and entry point"
```

---

### Task 6: systemd unit file

**Files:**
- Create: `~/.config/systemd/user/soul-courier.service`

- [ ] **Step 1: Create systemd unit**

```ini
# ~/.config/systemd/user/soul-courier.service
[Unit]
Description=Soul Team Courier — Message delivery daemon
After=network.target

[Service]
Type=simple
ExecStart=/usr/bin/python3 %h/.claude/scripts/soul-courier.py
Restart=always
RestartSec=3
Environment=SOUL_TEAM_NAME=soul-team
Environment=PYTHONPATH=%h/.claude/scripts

CPUQuota=50%%
MemoryMax=256M

TimeoutStopSec=10
KillMode=mixed
KillSignal=SIGTERM

StandardOutput=journal
StandardError=journal
SyslogIdentifier=soul-courier

[Install]
WantedBy=default.target
```

- [ ] **Step 2: Enable the service**

```bash
mkdir -p ~/.config/systemd/user
# (copy file above)
systemctl --user daemon-reload
systemctl --user enable soul-courier.service
```

- [ ] **Step 3: Verify unit loads without error**

Run: `systemctl --user status soul-courier.service`
Expected: Loaded but inactive (will be started by soul-team.sh)

- [ ] **Step 4: Commit**

```bash
git add ~/.config/systemd/user/soul-courier.service
git commit -m "feat(courier): add systemd user service unit"
```

---

### Task 7: Modify soul-team.sh — Cleanup, courier start, --continue

**Files:**
- Modify: `~/.claude/scripts/soul-team.sh:370-388` (replace sidecar/bridge launch)
- Modify: `~/.claude/scripts/soul-team.sh` (add --continue at top, add cleanup_stale)

- [ ] **Step 1: Read current soul-team.sh to identify exact edit locations**

Run: `cat -n ~/.claude/scripts/soul-team.sh | head -30` (find script start)
Run: `grep -n "sidecar\|bridge\|cleanup" ~/.claude/scripts/soul-team.sh`

- [ ] **Step 2: Add cleanup_stale function near top of script (after variable declarations)**

Add `cleanup_stale()` function after the initial variable declarations block. This function kills old sidecars, stops old courier/bridge, kills old tmux session, cleans seen logs + interrupt locks, and resets queues.

See spec section "soul-team.sh — Launcher Changes" for exact function body. Additionally add cleanup for interrupt lock files:
```bash
rm -f ~/.clawteam/teams/soul-team/sidecar/*-interrupt.lock
```

- [ ] **Step 3: Add --continue handler at top of script (before fresh launch logic)**

Add the `--continue` block right after argument parsing. See spec section "Continue Mode" for exact implementation.

Ensure `get_agent_launch_cmd()` function is implemented — it reads the agent launch command from the existing AGENTS array in soul-team.sh.

- [ ] **Step 4: Replace sidecar spawn loop (lines ~370-382) with courier start**

Replace:
```bash
# ── Start sidecars for selected agents + team-lead ───────────────────────
echo "  Starting soul sidecars..."
for agent in team-lead "${SELECTED_AGENTS[@]}"; do
    ...
done
echo "  All sidecars launched."
```

With:
```bash
# ── Start courier daemon (replaces sidecars + bridge) ─────────────────────
echo "  Starting soul-courier daemon..."
systemctl --user daemon-reload 2>/dev/null || true
systemctl --user start soul-courier.service 2>/dev/null && \
    echo "  Courier: started." || \
    echo "  Courier: could not start via systemd — launching directly..."
if ! systemctl --user is-active soul-courier.service >/dev/null 2>&1; then
    PYTHONPATH="$HOME/.claude/scripts" python3 "$HOME/.claude/scripts/soul-courier.py" \
        >> "$HOME/.claude/logs/soul-courier.log" 2>&1 &
    echo "  Courier PID: $!"
fi
```

- [ ] **Step 5: Remove bridge launch (lines ~385-388)**

Remove:
```bash
echo "  Starting soul-bridge daemon..."
python3 "$HOME/.claude/scripts/soul-bridge.py" >> "$HOME/.claude/logs/soul-bridge.log" 2>&1 &
BRIDGE_PID=$!
echo "  Bridge PID: $BRIDGE_PID"
```

- [ ] **Step 6: Call cleanup_stale at start of fresh launch path**

Add `cleanup_stale` call before the tmux session creation.

- [ ] **Step 7: Test --continue with no session**

Run: `bash ~/.claude/scripts/soul-team.sh --continue`
Expected: "No soul-team session found. Run 'soul-team' to start fresh."

- [ ] **Step 8: Commit**

```bash
git add ~/.claude/scripts/soul-team.sh
git commit -m "feat(courier): update launcher with cleanup, courier start, --continue"
```

---

### Task 8: Retire old files and integration test

**Files:**
- Rename: `~/.claude/scripts/soul-sidecar.sh` → `soul-sidecar.sh.bak`
- Rename: `~/.claude/scripts/soul-sidecar-wrapper.sh` → `soul-sidecar-wrapper.sh.bak`
- Rename: `~/.claude/scripts/soul-bridge.py` → `soul-bridge.py.bak`

- [ ] **Step 1: Rename retired files**

```bash
cd ~/.claude/scripts
mv soul-sidecar.sh soul-sidecar.sh.bak
mv soul-sidecar-wrapper.sh soul-sidecar-wrapper.sh.bak
mv soul-bridge.py soul-bridge.py.bak
```

- [ ] **Step 2: Run all unit tests**

```bash
cd ~/.claude/scripts && python3 -m pytest soul_courier/ -v
```

Expected: ALL PASS

- [ ] **Step 3: Manual integration test — start courier standalone**

```bash
# Start courier manually (not via systemd) to verify it works
PYTHONPATH=~/.claude/scripts python3 ~/.claude/scripts/soul-courier.py &
COURIER_PID=$!
sleep 3

# Send a test message via clawteam
clawteam inbox send soul-team team-lead "courier integration test" --from testbot

# Check logs
sleep 2
journalctl --user -u soul-courier --no-pager -n 20 2>/dev/null || echo "Check stdout"

# Cleanup
kill $COURIER_PID
```

- [ ] **Step 4: Manual integration test — verify message delivery to a running agent**

This test requires a running soul-team session. Run after next `soul-team` launch:

```bash
# After soul-team is running:
# 1. Send message to an idle agent
clawteam inbox send soul-team fury "Integration test from courier" --from team-lead

# 2. Wait 5s, then check fury's pane
tmux capture-pane -p -t $(jq -r '.fury' ~/.clawteam/teams/soul-team/panes.json) -S -10

# 3. Verify "Integration test from courier" appears in output
```

- [ ] **Step 5: Commit retirement**

```bash
cd ~/.claude/scripts
git add soul-sidecar.sh.bak soul-sidecar-wrapper.sh.bak soul-bridge.py.bak
git commit -m "chore(courier): retire old sidecar and bridge scripts"
```

---

### Task Summary

| Task | Component | Est. LOC | Dependencies |
|------|-----------|----------|--------------|
| 1 | MessageQueue | ~80 | None |
| 2 | PaneManager | ~150 | None |
| 3 | MessageFormatter | ~120 | None |
| 4 | InboxWatcher | ~40 | None |
| 5 | CourierDaemon | ~300 | Tasks 1-4 |
| 6 | systemd unit | ~20 | Task 5 |
| 7 | soul-team.sh mods | ~60 | Task 6 |
| 8 | Retire + integration test | ~10 | Task 7 |

Tasks 1-4 are independent and can run in parallel. Tasks 5-8 are sequential.
