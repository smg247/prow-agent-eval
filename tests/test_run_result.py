"""Tests for solve run_result loading."""

import json
import tempfile
from pathlib import Path

from prow_agent_eval.run_result import (
    SOLVE_RUN_RESULT_FILENAME,
    load_run_result,
    write_run_result,
)


def test_load_from_shared_solve_file():
    shared = Path(tempfile.mkdtemp())
    run_dir = Path(tempfile.mkdtemp())
    payload = {
        "model": "claude-opus-4-6",
        "cost_usd": 3.5,
        "num_turns": 10,
        "duration_s": 120.0,
        "agent": "claude-code",
    }
    (shared / SOLVE_RUN_RESULT_FILENAME).write_text(json.dumps(payload))
    result = load_run_result(run_dir, str(shared))
    assert result["model"] == "claude-opus-4-6"
    assert result["cost_usd"] == 3.5


def test_load_prefers_existing_run_result():
    shared = Path(tempfile.mkdtemp())
    run_dir = Path(tempfile.mkdtemp())
    (run_dir / "run_result.json").write_text(
        json.dumps({"model": "from-run-dir", "cost_usd": 1.0, "num_turns": 1})
    )
    (shared / SOLVE_RUN_RESULT_FILENAME).write_text(
        json.dumps({"model": "from-shared", "cost_usd": 9.0, "num_turns": 9})
    )
    result = load_run_result(run_dir, str(shared))
    assert result["model"] == "from-run-dir"


def test_load_returns_empty_when_missing():
    run_dir = Path(tempfile.mkdtemp())
    assert load_run_result(run_dir, None) == {}
    assert load_run_result(run_dir, tempfile.mkdtemp()) == {}


def test_write_run_result_skips_empty():
    run_dir = Path(tempfile.mkdtemp())
    write_run_result(run_dir, {})
    assert not (run_dir / "run_result.json").exists()

    write_run_result(
        run_dir,
        {"model": "claude-opus-4-6", "cost_usd": 1.0, "num_turns": 1, "duration_s": 10},
    )
    assert (run_dir / "run_result.json").is_file()
