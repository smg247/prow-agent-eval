"""Tests for git helpers."""

import subprocess
import tempfile

from prow_agent_eval.git import (
    DiffResult,
    eval_branch_name,
    short_sha,
    Repo,
)
from prow_agent_eval.git import _parse_name_only  # noqa: PLC2701


def test_short_sha():
    assert short_sha("abcdef123456", 8) == "abcdef12"
    assert short_sha("abc", 8) == "abc"
    assert short_sha("", 8) == ""


def test_parse_name_only():
    assert _parse_name_only("a.go\nb.go\n\n") == ["a.go", "b.go"]


def test_eval_branch_name():
    name = eval_branch_name("case-001")
    parts = name.split("-eval-", 1)
    assert parts[0] == "case-001"
    assert len(parts[1]) > 0


def test_repo_clone_and_diff_against():
    with tempfile.TemporaryDirectory() as parent:
        bare = f"{parent}/bare.git"
        work = f"{parent}/work"
        clone = f"{parent}/clone"
        subprocess.run(["git", "init", "--bare", bare], check=True, capture_output=True)
        subprocess.run(["git", "clone", bare, work], check=True, capture_output=True)
        subprocess.run(["git", "checkout", "-b", "main"], cwd=work, check=True, capture_output=True)
        with open(f"{work}/file.txt", "w") as f:
            f.write("hello")
        subprocess.run(["git", "add", "file.txt"], cwd=work, check=True, capture_output=True)
        subprocess.run(
            ["git", "commit", "-m", "init"],
            cwd=work,
            check=True,
            capture_output=True,
            env={"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@t.com", "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@t.com"},
        )
        subprocess.run(["git", "push", "-u", "origin", "main"], cwd=work, check=True, capture_output=True)

        base_sha = subprocess.run(
            ["git", "rev-parse", "HEAD"],
            cwd=work,
            check=True,
            capture_output=True,
            text=True,
        ).stdout.strip()
        with open(f"{work}/file.txt", "w") as f:
            f.write("hello world")
        subprocess.run(["git", "add", "file.txt"], cwd=work, check=True, capture_output=True)
        subprocess.run(
            ["git", "commit", "-m", "change"],
            cwd=work,
            check=True,
            capture_output=True,
            env={"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@t.com", "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@t.com"},
        )
        subprocess.run(["git", "push", "origin", "main"], cwd=work, check=True, capture_output=True)

        repo = Repo.clone(bare, clone)
        repo.fetch("origin", "main")
        repo.checkout("main")
        diff = repo.diff_against(base_sha)
        assert isinstance(diff, DiffResult)
        assert "file.txt" in diff.changed_files
        assert "hello world" in diff.full_diff


def test_open_validated_rejects_non_repo():
    with tempfile.TemporaryDirectory() as directory:
        try:
            Repo.open_validated(directory)
            assert False, "expected RuntimeError"
        except RuntimeError as err:
            assert "not a git repository" in str(err)
