"""Augment harness HTML reports with prow-agent-eval PR and diff links."""

from __future__ import annotations

import json
import re
from dataclasses import dataclass
from html import escape
from pathlib import Path


@dataclass(frozen=True)
class CaseLinks:
    case_name: str
    pr_url: str | None = None
    pr_number: int = 0
    expected_diff_url: str | None = None


def _compare_url(repo: str, base_branch: str, head_branch: str) -> str | None:
    if not repo or not base_branch or not head_branch:
        return None
    return f"https://github.com/{repo}/compare/{base_branch}...{head_branch}"


def _pr_url(repo: str, pr_number: int) -> str | None:
    if not repo or pr_number <= 0:
        return None
    return f"https://github.com/{repo}/pull/{pr_number}"


def links_from_evidence(case_name: str, evidence: dict) -> CaseLinks:
    gh = evidence.get("github", {})
    repo = gh.get("repo", "")
    base = gh.get("base_branch", "")
    expected_branch = gh.get("expected_branch", "")
    pr_number = int(gh.get("pr_number", 0) or 0)
    return CaseLinks(
        case_name=case_name,
        pr_url=_pr_url(repo, pr_number),
        pr_number=pr_number,
        expected_diff_url=_compare_url(repo, base, expected_branch),
    )


def load_case_links(run_dir: Path) -> list[CaseLinks]:
    cases_dir = run_dir / "cases"
    if not cases_dir.is_dir():
        return []
    links: list[CaseLinks] = []
    for case_dir in sorted(cases_dir.iterdir()):
        if not case_dir.is_dir():
            continue
        evidence_path = case_dir / "evidence.json"
        if not evidence_path.is_file():
            continue
        evidence = json.loads(evidence_path.read_text())
        links.append(links_from_evidence(case_dir.name, evidence))
    return links


def _overview_table(links: list[CaseLinks]) -> str:
    if not links:
        return ""

    rows = []
    for link in links:
        pr_cell = "—"
        if link.pr_url and link.pr_number:
            pr_cell = (
                f'<a href="{escape(link.pr_url)}" target="_blank" rel="noopener">'
                f"#{link.pr_number}</a>"
            )
        expected_cell = "—"
        if link.expected_diff_url:
            expected_cell = (
                f'<a href="{escape(link.expected_diff_url)}" target="_blank" rel="noopener">'
                "expected diff</a>"
            )
        rows.append(
            f"<tr><td>{escape(link.case_name)}</td>"
            f"<td>{pr_cell}</td>"
            f"<td>{expected_cell}</td></tr>"
        )

    return (
        "<h2>Case Links</h2>\n"
        "<table>\n"
        "<tr><th>Case</th><th>PR</th><th>Expected diff</th></tr>\n"
        + "\n".join(rows)
        + "\n</table>\n"
    )


_RUN_CONFIG_KEYS = (
    "model",
    "subagent_model",
    "effort",
    "agent",
    "agent_version",
    "duration_s",
    "cost_usd",
    "num_turns",
    "exit_code",
)


def run_result_has_metadata(run_result: dict) -> bool:
    """True when run_result contains harness agent-run metadata for the report."""
    if not run_result:
        return False
    for key in _RUN_CONFIG_KEYS:
        if run_result.get(key) not in (None, ""):
            return True
        if (run_result.get("eval_params") or {}).get(key) not in (None, ""):
            return True
    if run_result.get("token_usage") or run_result.get("per_model_usage"):
        return True
    if run_result.get("eval_params"):
        return True
    return False


def strip_empty_run_config(html: str) -> str:
    """Remove the Run Configuration section when it would only show placeholders."""
    pattern = r'<section class="section">\s*<h2>Run Configuration</h2>.*?</section>\s*'
    return re.sub(pattern, "", html, count=1, flags=re.DOTALL)


def enrich_html_report(
    html: str, run_dir: Path, run_result: dict | None = None
) -> str:
    """Insert case links and drop empty harness sections for judge-only evals."""
    if not run_result_has_metadata(run_result or {}):
        html = strip_empty_run_config(html)
    links = load_case_links(run_dir)
    if not links:
        return html
    overview = _overview_table(links)
    marker = '<h2 class="section-heading">Per-Case Details</h2>'
    if marker in html:
        return html.replace(marker, overview + marker, 1)
    return overview + html
