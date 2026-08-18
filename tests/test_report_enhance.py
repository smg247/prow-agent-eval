"""Tests for HTML report link enrichment."""

from pathlib import Path

from prow_agent_eval.report_enhance import enrich_html_report, links_from_evidence


def test_links_from_evidence():
    evidence = {
        "github": {
            "repo": "acme/widget",
            "base_branch": "eval/case-001-base",
            "expected_branch": "golden-fix",
            "agent_branch": "claude/fix",
            "pr_number": 42,
        }
    }
    links = links_from_evidence("case-001", evidence)
    assert links.pr_url == "https://github.com/acme/widget/pull/42"
    assert links.expected_diff_url == (
        "https://github.com/acme/widget/compare/eval/case-001-base...golden-fix"
    )
    assert links.agent_diff_url == (
        "https://github.com/acme/widget/compare/eval/case-001-base...claude/fix"
    )


def test_enrich_html_report_inserts_overview():
    run_dir = Path(__import__("tempfile").mkdtemp())
    case_dir = run_dir / "cases" / "case-001"
    case_dir.mkdir(parents=True)
    (case_dir / "evidence.json").write_text(
        """{
  "github": {
    "repo": "acme/widget",
    "base_branch": "main",
    "expected_branch": "golden",
    "agent_branch": "feature",
    "pr_number": 7
  }
}"""
    )
    html = "<h2 class=\"section-heading\">Per-Case Details</h2>\n<p>details</p>"
    out = enrich_html_report(html, run_dir)
    assert "Case Links" in out
    assert "https://github.com/acme/widget/pull/7" in out
    assert "expected diff" in out
    assert "agent diff" in out
    assert out.index("Case Links") < out.index("Per-Case Details")
