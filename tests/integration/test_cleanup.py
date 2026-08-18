"""Integration tests for cleanup command."""

import os

import responses
from click.testing import CliRunner

from prow_agent_eval.cli import main
from prow_agent_eval.github import GitHubClient
from prow_agent_eval.shared import CaseMetadata, write_case_list, write_case_metadata
from tests.integration.conftest import GitHubMock


@responses.activate
def test_cleanup_closes_pr_and_deletes_branch():
    mock = GitHubMock()
    mock.pr_number = 99
    mock.register(responses)

    shared_dir = __import__("tempfile").mkdtemp()
    write_case_list(shared_dir, ["case-001"])
    write_case_metadata(
        shared_dir,
        CaseMetadata(
            case_name="case-001",
            pr_number=99,
            head_branch="case-001-eval-20260101-120000",
            base_branch="main",
            repo="acme/widget",
        ),
    )

    os.environ["GITHUB_TOKEN"] = "test-token"
    runner = CliRunner()
    result = runner.invoke(
        main,
        [
            "cleanup",
            "--shared-dir", shared_dir,
            "--case", "case-001",
            "--token", "test-token",
        ],
    )
    assert result.exit_code == 0, result.output
    assert mock.has_call("PATCH", "/pulls/99")
    assert mock.has_call("DELETE", "/git/ref/heads/case-001-eval-20260101-120000")
