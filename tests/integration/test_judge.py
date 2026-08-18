"""Integration tests for judge command."""

import json
import os
import xml.etree.ElementTree as ET

import responses
from click.testing import CliRunner

from prow_agent_eval.cli import main
from prow_agent_eval.github import GitHubClient
from prow_agent_eval.shared import CaseMetadata, write_case_list, write_case_metadata, write_file
from tests.integration.conftest import fixture_dir, setup_bare_origin


def _seed_followup_shared(shared_dir: str, fixture_sha: str) -> None:
    write_case_list(shared_dir, ["case-001"])
    write_case_metadata(
        shared_dir,
        CaseMetadata(
            case_name="case-001",
            pr_number=42,
            head_branch="agent-work",
            base_branch="main",
            fixture_head_sha=fixture_sha,
            jira_issue_key="TRT-9001",
            repo="acme/widget",
            bot_login="test-bot",
        ),
    )
    posted = {
        "comment-001": {
            "github_id": 1001,
            "category": "valid_actionable",
            "created_at": "2026-01-01T00:00:00Z",
        },
        "comment-002": {
            "github_id": 1002,
            "category": "scope_creep",
            "created_at": "2026-01-01T00:00:01Z",
        },
    }
    write_file(shared_dir, "case-001.comment-map.json", json.dumps(posted))
    write_file(shared_dir, "case-001.eval-case", "case-001")


def _seed_solve_shared(shared_dir: str, fixture_sha: str) -> None:
    write_case_list(shared_dir, ["case-001"])
    write_case_metadata(
        shared_dir,
        CaseMetadata(
            case_name="case-001",
            pr_number=7,
            head_branch="claude/fix-trt-9002",
            base_branch="main",
            fixture_head_sha=fixture_sha,
            jira_issue_key="TRT-9002",
            repo="acme/widget",
        ),
    )
    write_file(shared_dir, "case-001.eval-case", "case-001")
    write_file(shared_dir, "case-001.eval-expected-branch", "golden-fix")


@responses.activate
def test_judge_followup():
    reconcile_fix = "package controller\n\nfunc Reconcile() { /* fixed */ }\n"
    bare, shas = setup_bare_origin({
        "main": {
            "README.md": "# widget\n",
            "pkg/api/server.go": "package api\n\nfunc Serve() {}\n",
        },
        "agent-work": {
            "README.md": "# widget\n",
            "pkg/api/server.go": "package api\n\nfunc Serve() { /* retries */ }\n",
        },
    })

    from tests.integration.conftest import GitHubMock

    mock = GitHubMock()
    mock.pr_number = 42
    mock.pr_body = "Fixes review feedback on retries"
    mock.pr_head = "agent-work"
    mock.issue_comments = [
        {
            "id": 1001,
            "body": "Please add retries around the API client.",
            "created_at": "2026-01-01T00:00:00Z",
            "user": {"login": mock.bot_login},
        },
        {
            "id": 1002,
            "body": "Add pagination support too?",
            "created_at": "2026-01-01T00:00:01Z",
            "user": {"login": mock.bot_login},
        },
    ]
    mock.register(responses)

    shared_dir = __import__("tempfile").mkdtemp()
    artifact_dir = __import__("tempfile").mkdtemp()
    _seed_followup_shared(shared_dir, shas["main"])

    eval_yaml = str(fixture_dir("followup") / "eval.yaml")
    os.environ["GITHUB_TOKEN"] = "test-token"

    from unittest.mock import patch

    original_init = GitHubClient.__init__

    def patched_init(self, token, repo_full_name, api_url=None, clone_url=None):
        original_init(self, token, repo_full_name, api_url=api_url, clone_url=bare)

    runner = CliRunner()
    with patch.object(GitHubClient, "__init__", patched_init):
        result = runner.invoke(
            main,
            [
                "judge",
                "--config", eval_yaml,
                "--shared-dir", shared_dir,
                "--artifact-dir", artifact_dir,
                "--repo", "acme/widget",
                "--mode", "followup",
                "--case", "case-001",
                "--token", "test-token",
            ],
        )

    assert result.exit_code == 0, result.output
    assert os.path.isfile(os.path.join(artifact_dir, "junit_followup-eval.xml"))
    assert os.path.isfile(os.path.join(artifact_dir, "eval-summary.html"))

    junit_data = open(os.path.join(artifact_dir, "junit_followup-eval.xml")).read()
    root = ET.fromstring(junit_data)
    suite = root.find("testsuite")
    assert suite is not None
    failures = int(suite.get("failures", "0"))
    assert failures >= 2


@responses.activate
def test_judge_solve():
    reconcile_fix = "package controller\n\nfunc Reconcile() { /* fixed */ }\n"
    bare, shas = setup_bare_origin({
        "main": {
            "README.md": "# widget\n",
            "pkg/controller/reconcile.go": "package controller\n\nfunc Reconcile() {}\n",
        },
        "golden-fix": {
            "README.md": "# widget\n",
            "pkg/controller/reconcile.go": reconcile_fix,
        },
        "claude/fix-trt-9002": {
            "README.md": "# widget\n",
            "pkg/controller/reconcile.go": reconcile_fix,
        },
    })

    from tests.integration.conftest import GitHubMock

    mock = GitHubMock()
    mock.pr_number = 7
    mock.pr_body = "Solve TRT-9002"
    mock.pr_head = "claude/fix-trt-9002"
    mock.register(responses)

    shared_dir = __import__("tempfile").mkdtemp()
    artifact_dir = __import__("tempfile").mkdtemp()
    _seed_solve_shared(shared_dir, shas["main"])

    eval_yaml = str(fixture_dir("solve") / "eval.yaml")
    os.environ["GITHUB_TOKEN"] = "test-token"

    from unittest.mock import patch

    original_init = GitHubClient.__init__

    def patched_init(self, token, repo_full_name, api_url=None, clone_url=None):
        original_init(self, token, repo_full_name, api_url=api_url, clone_url=bare)

    runner = CliRunner()
    with patch.object(GitHubClient, "__init__", patched_init):
        result = runner.invoke(
            main,
            [
                "judge",
                "--config", eval_yaml,
                "--shared-dir", shared_dir,
                "--artifact-dir", artifact_dir,
                "--repo", "acme/widget",
                "--mode", "solve",
                "--case", "case-001",
                "--token", "test-token",
            ],
        )

    assert result.exit_code == 0, result.output
    assert os.path.isfile(os.path.join(artifact_dir, "junit_solve-eval.xml"))
    assert os.path.isfile(os.path.join(artifact_dir, "eval-summary.html"))
