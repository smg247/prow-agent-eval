"""Integration tests for init command."""

import os

import responses
from click.testing import CliRunner

from prow_agent_eval.cli import main
from prow_agent_eval.github import GitHubClient
from prow_agent_eval.shared import read_case_list, read_case_metadata, read_file
from tests.integration.conftest import fixture_dir, GitHubMock, remote_has_branch, setup_bare_origin


@responses.activate
def test_init_followup():
    bare, _ = setup_bare_origin({
        "main": {
            "README.md": "# widget\n",
            "pkg/api/server.go": "package api\n\nfunc Serve() {}\n",
        },
        "fixture-head": {
            "README.md": "# widget\n",
            "pkg/api/server.go": "package api\n\nfunc Serve() { /* fixture */ }\n",
        },
    })

    mock = GitHubMock()
    mock.pr_body = "Automated eval PR for case: case-001\nJira: TRT-9001"
    mock.register(responses)

  # Patch clone URL via monkeypatch on GitHubClient after creation - use env?
    shared_dir = __import__("tempfile").mkdtemp()
    eval_yaml = str(fixture_dir("followup") / "eval.yaml")

    os.environ["GITHUB_TOKEN"] = "test-token"
    runner = CliRunner()
    # We need clone URL override - patch GitHubClient.clone_url by wrapping init
    from unittest.mock import patch

    original_init = GitHubClient.__init__

    def patched_init(self, token, repo_full_name, api_url=None, clone_url=None):
        original_init(self, token, repo_full_name, api_url=api_url, clone_url=bare)

    with patch.object(GitHubClient, "__init__", patched_init):
        result = runner.invoke(
            main,
            [
                "init",
                "--config", eval_yaml,
                "--shared-dir", shared_dir,
                "--repo", "acme/widget",
                "--mode", "followup",
                "--case", "case-001",
                "--token", "test-token",
            ],
        )

    assert result.exit_code == 0, result.output
    meta = read_case_metadata(shared_dir, "case-001")
    assert meta.repo == "acme/widget"
    assert meta.base_branch == "main"
    assert meta.jira_issue_key == "TRT-9001"
    assert meta.bot_login == "test-bot"
    assert meta.pr_number == 42
    assert meta.head_branch.startswith("case-001-eval-")
    assert meta.fixture_head_sha
    assert remote_has_branch(bare, meta.head_branch)
    assert read_case_list(shared_dir) == ["case-001"]
    assert os.path.isfile(os.path.join(shared_dir, "case-001.comment-map.json"))
    assert os.path.isfile(os.path.join(shared_dir, "case-001.jira-issue.json"))
    assert mock.has_call("POST", "/pulls")


def test_init_solve_metadata_only():
    shared_dir = __import__("tempfile").mkdtemp()
    eval_yaml = str(fixture_dir("solve") / "eval.yaml")
    runner = CliRunner()

    result = runner.invoke(
        main,
        [
            "init",
            "--config", eval_yaml,
            "--shared-dir", shared_dir,
            "--repo", "acme/widget",
            "--mode", "solve",
            "--case", "case-001",
            "--token", "test-token",
        ],
    )
    assert result.exit_code == 0, result.output

    meta = read_case_metadata(shared_dir, "case-001")
    assert meta.case_name == "case-001"
    assert meta.base_branch == "main"
    assert meta.jira_issue_key == "TRT-9002"
    assert meta.repo == "acme/widget"
    assert meta.head_branch == ""
    assert meta.pr_number == 0
    assert read_file(shared_dir, "case-001.eval-case") == "case-001"
    assert read_file(shared_dir, "case-001.eval-expected-branch") == "golden-fix"
