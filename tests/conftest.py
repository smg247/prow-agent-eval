"""Pytest configuration and shared fixtures."""

from __future__ import annotations

import os
import subprocess
from pathlib import Path

import pytest

VENDOR = Path(__file__).resolve().parent.parent / "vendor" / "agent-eval-harness"
HARNESS_TAG = "v1.39.3"
HARNESS_URL = "https://github.com/opendatahub-io/agent-eval-harness.git"


def _ensure_harness_repo() -> Path:
    score = VENDOR / "skills" / "eval-run" / "scripts" / "score.py"
    if score.is_file():
        return VENDOR
    if VENDOR.exists():
        import shutil
        shutil.rmtree(VENDOR)
    subprocess.run(
        [
            "git",
            "clone",
            "--depth",
            "1",
            "--branch",
            HARNESS_TAG,
            HARNESS_URL,
            str(VENDOR),
        ],
        check=True,
        capture_output=True,
    )
    if not score.is_file():
        raise RuntimeError(f"harness clone missing {score}")
    return VENDOR


@pytest.fixture(scope="session", autouse=True)
def harness_repo_root() -> Path:
    root = _ensure_harness_repo()
    os.environ["AGENT_EVAL_HARNESS_ROOT"] = str(root)
    return root
