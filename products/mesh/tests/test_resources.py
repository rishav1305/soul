"""Tests for live system resource collection.

These tests run against the REAL system -- no mocking needed.
They validate that get_cpu_info, get_memory_info, get_storage_info,
and get_system_snapshot return sane values from the actual OS.

The ``TestAndroid*`` classes use mocking to simulate Android/toybox
environments where GNU coreutils are not available.
"""

from __future__ import annotations

import asyncio
from unittest.mock import AsyncMock, patch, MagicMock

import pytest

from soul_mesh.resources import (
    get_cpu_info,
    get_memory_info,
    get_storage_info,
    get_system_snapshot,
    _parse_gnu_df,
    _parse_posix_df,
)


class TestGetCpuInfo:
    """CPU info from os.cpu_count and os.getloadavg."""

    @pytest.mark.asyncio
    async def test_cores_at_least_one(self):
        info = await get_cpu_info()
        assert info["cores"] >= 1

    @pytest.mark.asyncio
    async def test_cores_is_int(self):
        info = await get_cpu_info()
        assert isinstance(info["cores"], int)

    @pytest.mark.asyncio
    async def test_usage_percent_is_float(self):
        info = await get_cpu_info()
        assert isinstance(info["usage_percent"], float)

    @pytest.mark.asyncio
    async def test_usage_percent_non_negative(self):
        info = await get_cpu_info()
        assert info["usage_percent"] >= 0.0

    @pytest.mark.asyncio
    async def test_load_avg_1m_is_float(self):
        info = await get_cpu_info()
        assert isinstance(info["load_avg_1m"], float)

    @pytest.mark.asyncio
    async def test_load_avg_1m_non_negative(self):
        info = await get_cpu_info()
        assert info["load_avg_1m"] >= 0.0


class TestGetMemoryInfo:
    """Memory info parsed from /proc/meminfo (Linux) or sysctl (macOS)."""

    @pytest.mark.asyncio
    async def test_total_mb_positive(self):
        info = await get_memory_info()
        assert info["total_mb"] > 0

    @pytest.mark.asyncio
    async def test_total_mb_is_int(self):
        info = await get_memory_info()
        assert isinstance(info["total_mb"], int)

    @pytest.mark.asyncio
    async def test_available_mb_is_int(self):
        info = await get_memory_info()
        assert isinstance(info["available_mb"], int)

    @pytest.mark.asyncio
    async def test_available_mb_not_exceeds_total(self):
        info = await get_memory_info()
        assert info["available_mb"] <= info["total_mb"]

    @pytest.mark.asyncio
    async def test_used_percent_in_range(self):
        info = await get_memory_info()
        assert 0.0 <= info["used_percent"] <= 100.0

    @pytest.mark.asyncio
    async def test_used_percent_is_float(self):
        info = await get_memory_info()
        assert isinstance(info["used_percent"], float)


class TestGetStorageInfo:
    """Storage info from df subprocess."""

    @pytest.mark.asyncio
    async def test_mounts_is_list(self):
        info = await get_storage_info()
        assert isinstance(info["mounts"], list)

    @pytest.mark.asyncio
    async def test_at_least_one_mount(self):
        info = await get_storage_info()
        assert len(info["mounts"]) >= 1

    @pytest.mark.asyncio
    async def test_root_mount_present(self):
        info = await get_storage_info()
        paths = [m["path"] for m in info["mounts"]]
        assert "/" in paths

    @pytest.mark.asyncio
    async def test_mount_has_required_keys(self):
        info = await get_storage_info()
        for mount in info["mounts"]:
            assert "path" in mount
            assert "total_gb" in mount
            assert "free_gb" in mount

    @pytest.mark.asyncio
    async def test_mount_values_are_numeric(self):
        info = await get_storage_info()
        for mount in info["mounts"]:
            assert isinstance(mount["path"], str)
            assert isinstance(mount["total_gb"], (int, float))
            assert isinstance(mount["free_gb"], (int, float))

    @pytest.mark.asyncio
    async def test_free_not_exceeds_total(self):
        info = await get_storage_info()
        for mount in info["mounts"]:
            assert mount["free_gb"] <= mount["total_gb"]


class TestGetSystemSnapshot:
    """Full system snapshot combining cpu, memory, storage."""

    @pytest.mark.asyncio
    async def test_has_all_sections(self):
        snap = await get_system_snapshot()
        assert "cpu" in snap
        assert "memory" in snap
        assert "storage" in snap

    @pytest.mark.asyncio
    async def test_cpu_section_valid(self):
        snap = await get_system_snapshot()
        assert snap["cpu"]["cores"] >= 1
        assert isinstance(snap["cpu"]["usage_percent"], float)

    @pytest.mark.asyncio
    async def test_memory_section_valid(self):
        snap = await get_system_snapshot()
        assert snap["memory"]["total_mb"] > 0
        assert 0.0 <= snap["memory"]["used_percent"] <= 100.0

    @pytest.mark.asyncio
    async def test_storage_section_valid(self):
        snap = await get_system_snapshot()
        assert len(snap["storage"]["mounts"]) >= 1
        paths = [m["path"] for m in snap["storage"]["mounts"]]
        assert "/" in paths


# ---------------------------------------------------------------------------
# Android / POSIX fallback tests (mocked)
# ---------------------------------------------------------------------------

class TestParseGnuDf:
    """Unit tests for _parse_gnu_df (pure function)."""

    def test_parses_standard_output(self):
        output = (
            "Mounted on      Size  Avail\n"
            "/               50G    30G\n"
            "/home          100G    60G\n"
        )
        result = _parse_gnu_df(output)
        assert len(result["mounts"]) == 2
        assert result["mounts"][0] == {"path": "/", "total_gb": 50, "free_gb": 30}
        assert result["mounts"][1] == {"path": "/home", "total_gb": 100, "free_gb": 60}

    def test_skips_pseudo_filesystems(self):
        output = (
            "Mounted on      Size  Avail\n"
            "/               50G    30G\n"
            "/dev            1G     1G\n"
            "/proc           0G     0G\n"
        )
        result = _parse_gnu_df(output)
        assert len(result["mounts"]) == 1
        assert result["mounts"][0]["path"] == "/"


class TestParsePosixDf:
    """Unit tests for _parse_posix_df (pure function)."""

    def test_parses_standard_output(self):
        output = (
            "Filesystem     1K-blocks    Used Available Use% Mounted on\n"
            "/dev/sda1      52428800  20971520  31457280  40% /\n"
        )
        result = _parse_posix_df(output)
        assert len(result["mounts"]) == 1
        mount = result["mounts"][0]
        assert mount["path"] == "/"
        # 52428800 KB = 50 GB (integer division)
        assert mount["total_gb"] == 52428800 // (1024 * 1024)
        assert mount["free_gb"] == 31457280 // (1024 * 1024)

    def test_skips_pseudo_filesystems(self):
        output = (
            "Filesystem     1K-blocks    Used Available Use% Mounted on\n"
            "/dev/sda1      52428800  20971520  31457280  40% /\n"
            "tmpfs          1024000   0        1024000   0%  /run\n"
        )
        result = _parse_posix_df(output)
        assert len(result["mounts"]) == 1
        assert result["mounts"][0]["path"] == "/"

    def test_skips_short_lines(self):
        output = (
            "Filesystem     1K-blocks    Used Available Use% Mounted on\n"
            "short\n"
        )
        result = _parse_posix_df(output)
        assert result["mounts"] == []


class TestStorageLinuxFallback:
    """Test that _storage_linux falls back to POSIX df when GNU df fails."""

    @pytest.mark.asyncio
    async def test_uses_gnu_df_when_available(self):
        """When GNU df succeeds (rc=0), use its output."""
        from soul_mesh.resources import _storage_linux

        mock_proc = AsyncMock()
        mock_proc.communicate = AsyncMock(return_value=(
            b"Mounted on      Size  Avail\n/               50G    30G\n",
            b"",
        ))
        mock_proc.returncode = 0

        with patch("soul_mesh.resources.asyncio.create_subprocess_exec",
                    return_value=mock_proc) as mock_exec:
            result = await _storage_linux()

        assert len(result["mounts"]) == 1
        assert result["mounts"][0]["total_gb"] == 50
        # Should only call df once (GNU succeeded)
        mock_exec.assert_called_once()

    @pytest.mark.asyncio
    async def test_falls_back_to_posix_df(self):
        """When GNU df fails (rc!=0), fall back to df -k."""
        from soul_mesh.resources import _storage_linux

        gnu_proc = AsyncMock()
        gnu_proc.communicate = AsyncMock(return_value=(b"", b"df: invalid option"))
        gnu_proc.returncode = 1

        posix_proc = AsyncMock()
        posix_proc.communicate = AsyncMock(return_value=(
            b"Filesystem     1K-blocks    Used Available Use% Mounted on\n"
            b"/dev/sda1      104857600  41943040  62914560  40% /\n",
            b"",
        ))
        posix_proc.returncode = 0

        with patch("soul_mesh.resources.asyncio.create_subprocess_exec",
                    side_effect=[gnu_proc, posix_proc]):
            result = await _storage_linux()

        assert len(result["mounts"]) == 1
        assert result["mounts"][0]["path"] == "/"
        assert result["mounts"][0]["total_gb"] == 104857600 // (1024 * 1024)


class TestGetStorageMbFallback:
    """Test _get_storage_mb falls back from df -m to df -k."""

    @pytest.mark.asyncio
    async def test_uses_df_m_when_available(self):
        from soul_mesh.node import _get_storage_mb

        mock_proc = AsyncMock()
        mock_proc.communicate = AsyncMock(return_value=(
            b"Filesystem     1M-blocks  Used Available Use% Mounted on\n"
            b"/dev/sda1      102400     40960  61440    40% /\n",
            b"",
        ))
        mock_proc.returncode = 0

        with patch("soul_mesh.node.asyncio.create_subprocess_exec",
                    return_value=mock_proc):
            result = await _get_storage_mb()

        assert result == 102400

    @pytest.mark.asyncio
    async def test_falls_back_to_df_k(self):
        from soul_mesh.node import _get_storage_mb

        df_m_proc = AsyncMock()
        df_m_proc.communicate = AsyncMock(return_value=(b"", b""))
        df_m_proc.returncode = 1

        df_k_proc = AsyncMock()
        df_k_proc.communicate = AsyncMock(return_value=(
            b"Filesystem     1K-blocks  Used Available Use% Mounted on\n"
            b"/dev/sda1      104857600  41943040  62914560  40% /\n",
            b"",
        ))
        df_k_proc.returncode = 0

        with patch("soul_mesh.node.asyncio.create_subprocess_exec",
                    side_effect=[df_m_proc, df_k_proc]):
            result = await _get_storage_mb()

        # 104857600 KB // 1024 = 102400 MB
        assert result == 104857600 // 1024


class TestGetRamMbFallback:
    """Test _get_ram_mb falls back from free to /proc/meminfo."""

    @pytest.mark.asyncio
    async def test_uses_free_when_available(self):
        from soul_mesh.node import _get_ram_mb

        mock_proc = AsyncMock()
        mock_proc.communicate = AsyncMock(return_value=(
            b"              total        used        free\n"
            b"Mem:          16000        8000        8000\n",
            b"",
        ))
        mock_proc.returncode = 0

        with patch("soul_mesh.node._platform.system", return_value="Linux"), \
             patch("soul_mesh.node.asyncio.create_subprocess_exec",
                   return_value=mock_proc):
            result = await _get_ram_mb()

        assert result == 16000

    @pytest.mark.asyncio
    async def test_falls_back_to_proc_meminfo_on_file_not_found(self):
        from soul_mesh.node import _get_ram_mb

        meminfo = "MemTotal:       16384000 kB\nMemFree:         8192000 kB\n"

        with patch("soul_mesh.node._platform.system", return_value="Linux"), \
             patch("soul_mesh.node.asyncio.create_subprocess_exec",
                   side_effect=FileNotFoundError("free")), \
             patch("soul_mesh.node.asyncio.to_thread", return_value=meminfo):
            result = await _get_ram_mb()

        # 16384000 kB // 1024 = 16000 MB
        assert result == 16384000 // 1024

    @pytest.mark.asyncio
    async def test_falls_back_to_proc_meminfo_on_nonzero_exit(self):
        from soul_mesh.node import _get_ram_mb

        mock_proc = AsyncMock()
        mock_proc.communicate = AsyncMock(return_value=(b"", b""))
        mock_proc.returncode = 1

        meminfo = "MemTotal:       8192000 kB\nMemFree:         4096000 kB\n"

        with patch("soul_mesh.node._platform.system", return_value="Linux"), \
             patch("soul_mesh.node.asyncio.create_subprocess_exec",
                   return_value=mock_proc), \
             patch("soul_mesh.node.asyncio.to_thread", return_value=meminfo):
            result = await _get_ram_mb()

        assert result == 8192000 // 1024
