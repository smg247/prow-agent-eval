"""Harness scoring pipeline wrapper."""

from __future__ import annotations

import importlib.util
import logging
import os
from pathlib import Path

import yaml

from prow_agent_eval.junit import write_junit

logger = logging.getLogger(__name__)


def _harness_repo_root() -> Path:
    env = os.environ.get("AGENT_EVAL_HARNESS_ROOT")
    if env:
        root = Path(env)
        score_path = root / "skills" / "eval-run" / "scripts" / "score.py"
        if score_path.is_file():
            return root
        raise FileNotFoundError(f"Judge engine not found under AGENT_EVAL_HARNESS_ROOT: {score_path}")

    here = Path(__file__).resolve().parent
    candidates = [
        here.parent / "vendor" / "agent-eval-harness",
        here.parent.parent / "agent-eval-harness",
        Path("/opt/agent-eval-harness"),
    ]
    for candidate in candidates:
        score_path = candidate / "skills" / "eval-run" / "scripts" / "score.py"
        if score_path.is_file():
            return candidate

    raise FileNotFoundError(
        "agent-eval-harness skills not found. Set AGENT_EVAL_HARNESS_ROOT to a "
        "full harness repo clone (with skills/eval-run/scripts/score.py), or clone "
        "to vendor/agent-eval-harness"
    )


def _load_score_module():
    score_path = _harness_repo_root() / "skills" / "eval-run" / "scripts" / "score.py"
    spec = importlib.util.spec_from_file_location("agent_eval_score", score_path)
    if spec is None or spec.loader is None:
        raise ImportError(f"cannot load score module from {score_path}")
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    for required in ("load_judges", "score_cases", "detect_regressions", "_merge_summary"):
        if not hasattr(mod, required):
            raise AttributeError(f"score.py is missing {required}()")
    return mod


def _resolve_thresholds(thresholds, aggregated: dict) -> dict:
    """Map legacy threshold names to ``_py`` judges during Go/Python transition."""
    resolved: dict = {}
    for name, spec in thresholds.items():
        target = name
        if name not in aggregated and f"{name}_py" in aggregated:
            target = f"{name}_py"
        resolved[target] = spec
    return resolved


def _load_report_module():
    report_path = _harness_repo_root() / "skills" / "eval-run" / "scripts" / "report.py"
    if not report_path.is_file():
        raise FileNotFoundError(f"report module not found: {report_path}")
    spec = importlib.util.spec_from_file_location("agent_eval_report", report_path)
    if spec is None or spec.loader is None:
        raise ImportError(f"cannot load report module from {report_path}")
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    if not hasattr(mod, "generate_report"):
        raise AttributeError("report.py is missing generate_report()")
    return mod


def run(
    config,
    case_dirs: list[Path],
    artifact_dir: str,
    run_id: str | None = None,
) -> bool:
    """Run harness scoring pipeline and write JUnit + HTML report."""
    score = _load_score_module()
    report_mod = _load_report_module()

    eval_name = config.eval_name()
    run_id = run_id or eval_name
    runs_base = Path(artifact_dir) / "eval" / "runs"
    runs_dir = runs_base / eval_name
    run_dir = runs_dir / run_id
    run_dir.mkdir(parents=True, exist_ok=True)

    judges = score.load_judges(config, project_root=Path.cwd())
    results = score.score_cases(judges, case_dirs, config, run_id=run_id)

    score._merge_summary(run_id, "judges", results["aggregated"], runs_dir)
    score._merge_summary(run_id, "per_case", results["per_case"], runs_dir)

    thresholds = _resolve_thresholds(config.thresholds, results["aggregated"])
    regressions = score.detect_regressions(results["aggregated"], thresholds)

    summary_path = run_dir / "summary.yaml"
    summary = yaml.safe_load(summary_path.read_text()) if summary_path.is_file() else {}

    config_dict = yaml.safe_load(Path(config.config_path).read_text())
    html = report_mod.generate_report(
        config_dict, summary, run_result={}, run_dir=run_dir
    )
    html_path = Path(artifact_dir) / "eval-summary.html"
    html_path.write_text(html)
    os.chmod(html_path, 0o600)

    write_junit(artifact_dir, eval_name, results["per_case"])

    for reg in regressions:
        logger.warning(
            "threshold missed: %s actual=%s required=%s",
            reg.get("name"),
            reg.get("actual"),
            reg.get("required"),
        )

    return len(regressions) == 0
