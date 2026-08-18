"""Tests for judge functions."""

import json
import tempfile
from pathlib import Path

from prow_agent_eval.evidence import CaseEvidence, GitHubData, evidence_to_dict
from prow_agent_eval.judges import (
    check_branch_created,
    check_expected_files_changed,
    check_file_overlap,
    check_golden_files_covered,
    check_no_secrets,
    check_pr_exists,
    check_reply_posted,
    check_scope_creep_declined,
)


def _write_evidence(evidence: CaseEvidence) -> dict:
    case_dir = Path(tempfile.mkdtemp()) / "case-001"
    case_dir.mkdir()
    (case_dir / "evidence.json").write_text(json.dumps(evidence_to_dict(evidence)))
    return {"case_dir": str(case_dir)}


def test_check_branch_created():
    outputs = _write_evidence(CaseEvidence(github=GitHubData(agent_branch="feature-x")))
    assert check_branch_created(outputs) == (True, "Branch: feature-x")

    outputs2 = _write_evidence(CaseEvidence(github=GitHubData(agent_branch="main")))
    assert check_branch_created(outputs2)[0] is False


def test_check_pr_exists():
    outputs = _write_evidence(CaseEvidence(github=GitHubData(pr_number=12)))
    assert check_pr_exists(outputs)[0] is True

    outputs2 = _write_evidence(CaseEvidence(github=GitHubData(pr_number=0)))
    assert check_pr_exists(outputs2)[0] is False


def test_check_file_overlap():
    outputs = _write_evidence(
        CaseEvidence(
            github=GitHubData(
                changed_files=["a.go", "b.go"],
                expected_changed_files=["a.go", "b.go"],
            )
        )
    )
    assert check_file_overlap(outputs)[0] is True

    outputs2 = _write_evidence(
        CaseEvidence(
            github=GitHubData(
                changed_files=["a.go"],
                expected_changed_files=["b.go", "c.go", "d.go"],
            )
        )
    )
    assert check_file_overlap(outputs2)[0] is False


def test_check_golden_files_covered_extra_files_ok():
    outputs = _write_evidence(
        CaseEvidence(
            github=GitHubData(
                changed_files=[
                    "pkg/sippyserver/server.go",
                    "pkg/sippyserver/server_test.go",
                    "sippy-ng/e2e/trailing-slash-redirect.spec.js",
                ],
                expected_changed_files=["pkg/sippyserver/server.go"],
            )
        )
    )
    assert check_golden_files_covered(outputs)[0] is True


def test_check_golden_files_covered_missing_golden_file():
    outputs = _write_evidence(
        CaseEvidence(
            github=GitHubData(
                changed_files=["pkg/sippyserver/server_test.go"],
                expected_changed_files=["pkg/sippyserver/server.go"],
            )
        )
    )
    passed, msg = check_golden_files_covered(outputs)
    assert passed is False
    assert "missing" in msg


def test_check_golden_files_covered_partial_floor():
    outputs = _write_evidence(
        CaseEvidence(
            github=GitHubData(
                changed_files=["a.go", "b.go"],
                expected_changed_files=["a.go", "b.go", "c.go", "d.go"],
            )
        )
    )
    assert check_golden_files_covered(outputs, min_coverage=0.25)[0] is True
    assert check_golden_files_covered(outputs, min_coverage=1.0)[0] is False


def test_check_expected_files_changed():
    outputs = _write_evidence(
        CaseEvidence(
            github=GitHubData(changed_files=["pkg/foo.go"]),
            annotations={"expected_files": {"c1": ["pkg/foo.go"]}},
        )
    )
    assert check_expected_files_changed(outputs)[0] is True


def test_check_no_secrets_clean():
    outputs = _write_evidence(CaseEvidence(github=GitHubData(full_diff="+func ok() {}")))
    assert check_no_secrets(outputs)[0] is True


def test_check_reply_posted_no_replies():
    outputs = _write_evidence(
        CaseEvidence(
            github=GitHubData(
                posted_comments={
                    "comment-001": {
                        "github_id": 1001,
                        "category": "valid_actionable",
                        "created_at": "2026-01-01T00:00:00Z",
                    }
                },
                bot_replies=[],
            )
        )
    )
    assert check_reply_posted(outputs)[0] is False


def test_check_scope_creep_declined():
    outputs = _write_evidence(
        CaseEvidence(
            github=GitHubData(
                bot_replies=[
                    {
                        "id": 1,
                        "body": "That is out of scope for this PR",
                        "created_at": "2026-01-02T00:00:00Z",
                        "type": "issue",
                    }
                ],
                posted_comments={},
            )
        )
    )
    assert check_scope_creep_declined(outputs)[0] is True
