"""Augment harness HTML reports with prow-agent-eval PR and diff links."""

from __future__ import annotations

import json
from dataclasses import dataclass
from html import escape
from pathlib import Path


@dataclass(frozen=True)
class CaseLinks:
    case_name: str
    pr_url: str | None = None
    pr_number: int = 0
    expected_diff_url: str | None = None
    agent_diff_url: str | None = None


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
    agent_branch = gh.get("agent_branch", "")
    expected_branch = gh.get("expected_branch", "")
    pr_number = int(gh.get("pr_number", 0) or 0)
    return CaseLinks(
        case_name=case_name,
        pr_url=_pr_url(repo, pr_number),
        pr_number=pr_number,
        expected_diff_url=_compare_url(repo, base, expected_branch),
        agent_diff_url=_compare_url(repo, base, agent_branch),
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
        agent_cell = "—"
        if link.agent_diff_url:
            agent_cell = (
                f'<a href="{escape(link.agent_diff_url)}" target="_blank" rel="noopener">'
                "agent diff</a>"
            )
        rows.append(
            f"<tr><td>{escape(link.case_name)}</td>"
            f"<td>{pr_cell}</td>"
            f"<td>{expected_cell}</td>"
            f"<td>{agent_cell}</td></tr>"
        )

    return (
        "<h2>Case Links</h2>\n"
        "<table>\n"
        "<tr><th>Case</th><th>PR</th><th>Expected diff</th><th>Agent diff</th></tr>\n"
        + "\n".join(rows)
        + "\n</table>\n"
    )


def enrich_html_report(html: str, run_dir: Path) -> str:
    """Insert overview links table before per-case details."""
    links = load_case_links(run_dir)
    if not links:
        return html
    overview = _overview_table(links)
    marker = '<h2 class="section-heading">Per-Case Details</h2>'
    if marker in html:
        return html.replace(marker, overview + marker, 1)
    return overview + html
