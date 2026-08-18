"""Integration test helpers: bare git repos and GitHub API mocks."""

from __future__ import annotations

import json
import os
import re
import subprocess
import tempfile
from pathlib import Path
from typing import Any

import responses


_GH = re.compile(r"https://api\.github\.com(?::443)?")


def fixture_dir(name: str) -> Path:
    return Path(__file__).resolve().parent / "testdata" / name


def setup_bare_origin(branches: dict[str, dict[str, str]]) -> tuple[str, dict[str, str]]:
    """Create a bare git repo with named branches. Returns (bare_path, shas)."""
    root = tempfile.mkdtemp(prefix="prow-agent-eval-bare-")
    work = os.path.join(root, "work")
    bare = os.path.join(root, "origin.git")

    subprocess.run(["git", "init", "-b", "main", work], check=True, capture_output=True)
    subprocess.run(["git", "config", "user.email", "test@example.com"], cwd=work, check=True)
    subprocess.run(["git", "config", "user.name", "Test"], cwd=work, check=True)

    shas: dict[str, str] = {}
    main_files = branches.get("main", {"README.md": "# widget\n"})
    _write_files(work, main_files)
    subprocess.run(["git", "add", "."], cwd=work, check=True, capture_output=True)
    subprocess.run(["git", "commit", "-m", "initial main"], cwd=work, check=True, capture_output=True)
    shas["main"] = subprocess.run(
        ["git", "rev-parse", "HEAD"], cwd=work, check=True, capture_output=True, text=True
    ).stdout.strip()

    for name, files in branches.items():
        if name == "main":
            continue
        subprocess.run(["git", "checkout", "-b", name, "main"], cwd=work, check=True, capture_output=True)
        _write_files(work, files)
        subprocess.run(["git", "add", "."], cwd=work, check=True, capture_output=True)
        subprocess.run(
            ["git", "commit", "-m", f"commit on {name}"], cwd=work, check=True, capture_output=True
        )
        shas[name] = subprocess.run(
            ["git", "rev-parse", "HEAD"], cwd=work, check=True, capture_output=True, text=True
        ).stdout.strip()
        subprocess.run(["git", "checkout", "main"], cwd=work, check=True, capture_output=True)

    subprocess.run(["git", "clone", "--bare", work, bare], check=True, capture_output=True)
    subprocess.run(
        ["git", "config", "receive.denyCurrentBranch", "ignore"],
        cwd=bare,
        check=True,
        capture_output=True,
    )
    return bare, shas


def _write_files(root: str, files: dict[str, str]) -> None:
    for rel_path, content in files.items():
        full = os.path.join(root, rel_path)
        os.makedirs(os.path.dirname(full), exist_ok=True)
        with open(full, "w") as f:
            f.write(content)


def remote_has_branch(bare_path: str, branch: str) -> bool:
    return subprocess.run(
        ["git", "--git-dir", bare_path, "rev-parse", "--verify", branch],
        capture_output=True,
    ).returncode == 0


class GitHubMock:
    def __init__(self) -> None:
        self.bot_login = "test-bot"
        self.pr_number = 42
        self.pr_body = "Automated eval PR for case: case-001\nJira: TRT-9001"
        self.pr_head = "case-001-eval-branch"
        self.pr_state = "open"
        self.pr_sha = "abc123def456"
        self.issue_comments: list[dict[str, Any]] | None = None
        self._next_comment_id = 1000
        self.calls: list[tuple[str, str]] = []

    def register(self, rsps: responses.RequestsMock, owner: str = "acme", repo: str = "widget") -> None:
        base = f"/repos/{owner}/{repo}"

        def _track(method: str, url: str) -> None:
            self.calls.append((method, url))

        def _user(req):
            _track("GET", req.url)
            return (200, {}, json.dumps({"login": self.bot_login}))

        def _repo(req):
            _track("GET", req.url)
            return (200, {}, json.dumps({"name": repo, "full_name": f"{owner}/{repo}"}))

        def _empty_review_comments(req):
            _track("GET", req.url)
            return (200, {}, "[]")

        rsps.add_callback(
            responses.GET,
            re.compile(_GH.pattern + r"/user$"),
            callback=_user,
            content_type="application/json",
        )

        rsps.add_callback(
            responses.GET,
            re.compile(_GH.pattern + re.escape(base) + r"$"),
            callback=_repo,
            content_type="application/json",
        )

        rsps.add_callback(
            responses.POST,
            re.compile(_GH.pattern + re.escape(base) + r"/pulls$"),
            callback=self._create_pr,
            content_type="application/json",
        )

        rsps.add_callback(
            responses.GET,
            re.compile(_GH.pattern + re.escape(base) + f"/pulls/{self.pr_number}$"),
            callback=self._get_pr,
            content_type="application/json",
        )

        rsps.add_callback(
            responses.GET,
            re.compile(_GH.pattern + re.escape(base) + r"/pulls$"),
            callback=self._list_prs,
            content_type="application/json",
        )

        rsps.add_callback(
            responses.POST,
            re.compile(_GH.pattern + re.escape(base) + f"/issues/{self.pr_number}/comments$"),
            callback=self._post_issue_comment,
            content_type="application/json",
        )

        rsps.add_callback(
            responses.POST,
            re.compile(_GH.pattern + re.escape(base) + f"/pulls/{self.pr_number}/comments$"),
            callback=self._post_review_comment,
            content_type="application/json",
        )

        rsps.add_callback(
            responses.GET,
            re.compile(_GH.pattern + re.escape(base) + f"/issues/{self.pr_number}/comments$"),
            callback=self._list_issue_comments,
            content_type="application/json",
        )

        rsps.add_callback(
            responses.GET,
            re.compile(_GH.pattern + re.escape(base) + f"/pulls/{self.pr_number}/comments$"),
            callback=_empty_review_comments,
            content_type="application/json",
        )

        rsps.add_callback(
            responses.PATCH,
            re.compile(_GH.pattern + re.escape(base) + f"/pulls/{self.pr_number}$"),
            callback=self._close_pr,
            content_type="application/json",
        )

        def _git_ref(req):
            _track("GET", req.url)
            return (
                200,
                {},
                json.dumps({"ref": "refs/heads/branch", "object": {"sha": self.pr_sha}}),
            )

        def _issue(req):
            _track("GET", req.url)
            return (200, {}, json.dumps({"number": self.pr_number}))

        rsps.add_callback(
            responses.GET,
            re.compile(_GH.pattern + re.escape(base) + f"/issues/{self.pr_number}$"),
            callback=_issue,
            content_type="application/json",
        )

        rsps.add_callback(
            responses.GET,
            re.compile(_GH.pattern + re.escape(base) + r"/git/ref/heads/.*"),
            callback=_git_ref,
            content_type="application/json",
        )

        rsps.add_callback(
            responses.DELETE,
            re.compile(_GH.pattern + re.escape(base) + r"/git/ref/heads/.*"),
            callback=self._delete_ref,
            content_type="application/json",
        )

    def _create_pr(self, request):
        self.calls.append(("POST", request.url))
        body = {
            "number": self.pr_number,
            "html_url": f"https://github.com/acme/widget/pull/{self.pr_number}",
            "state": self.pr_state,
            "body": self.pr_body,
            "head": {"ref": self.pr_head, "sha": self.pr_sha},
        }
        return (201, {}, json.dumps(body))

    def _get_pr(self, request):
        self.calls.append(("GET", request.url))
        body = {
            "number": self.pr_number,
            "state": self.pr_state,
            "body": self.pr_body,
            "head": {"ref": self.pr_head, "sha": self.pr_sha},
        }
        return (200, {}, json.dumps(body))

    def _list_prs(self, request):
        self.calls.append(("GET", request.url))
        body = [{
            "number": self.pr_number,
            "state": self.pr_state,
            "body": self.pr_body,
            "head": {"ref": self.pr_head, "sha": self.pr_sha},
        }]
        return (200, {}, json.dumps(body))

    def _post_issue_comment(self, request):
        self.calls.append(("POST", request.url))
        self._next_comment_id += 1
        body = {
            "id": self._next_comment_id,
            "body": "seeded",
            "created_at": "2026-01-01T00:00:00Z",
            "user": {"login": self.bot_login},
        }
        return (201, {}, json.dumps(body))

    def _post_review_comment(self, request):
        self.calls.append(("POST", request.url))
        self._next_comment_id += 1
        body = {
            "id": self._next_comment_id,
            "body": "seeded inline",
            "path": "pkg/api/server.go",
            "created_at": "2026-01-01T00:00:00Z",
            "user": {"login": self.bot_login},
        }
        return (201, {}, json.dumps(body))

    def _list_issue_comments(self, request):
        self.calls.append(("GET", request.url))
        if self.issue_comments is not None:
            return (200, {}, json.dumps(self.issue_comments))
        body = [{
            "id": 2001,
            "body": "bot reply looks fine",
            "created_at": "2026-01-01T00:00:00Z",
            "user": {"login": self.bot_login},
        }]
        return (200, {}, json.dumps(body))

    def _close_pr(self, request):
        self.calls.append(("PATCH", request.url))
        return (200, {}, json.dumps({"number": self.pr_number, "state": "closed"}))

    def _delete_ref(self, request):
        self.calls.append(("DELETE", request.url))
        return (204, {}, "")

    def has_call(self, method: str, path_substr: str) -> bool:
        return any(c[0] == method and path_substr in c[1] for c in self.calls)
