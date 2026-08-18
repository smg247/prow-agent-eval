"""Load harness run_result.json for eval HTML Run Configuration.

The solve step publishes ``{shared_dir}/eval-solve-run-result.json`` once OTEL /
agentic-ci metrics are wired (same shape as harness ``run_result.json``). Judge
only copies that into the eval run dir when present.
"""

from __future__ import annotations

import json
import logging
import os
from pathlib import Path

from prow_agent_eval.report_enhance import run_result_has_metadata

logger = logging.getLogger(__name__)

SOLVE_RUN_RESULT_FILENAME = "eval-solve-run-result.json"


def load_run_result(run_dir: Path, shared_dir: str | None) -> dict:
    """Return run metadata for the harness HTML report, if any source has it."""
    existing_path = run_dir / "run_result.json"
    if existing_path.is_file():
        try:
            existing = json.loads(existing_path.read_text())
            if run_result_has_metadata(existing):
                return existing
        except json.JSONDecodeError:
            logger.warning("invalid run_result.json in %s", run_dir)

    if not shared_dir:
        return {}

    solve_path = Path(shared_dir) / SOLVE_RUN_RESULT_FILENAME
    if not solve_path.is_file():
        return {}

    try:
        data = json.loads(solve_path.read_text())
    except json.JSONDecodeError:
        logger.warning("invalid %s", solve_path)
        return {}

    if run_result_has_metadata(data):
        return data
    return {}


def write_run_result(run_dir: Path, run_result: dict) -> None:
    if not run_result_has_metadata(run_result):
        return
    path = run_dir / "run_result.json"
    path.write_text(json.dumps(run_result, indent=2) + "\n")
    os.chmod(path, 0o600)
