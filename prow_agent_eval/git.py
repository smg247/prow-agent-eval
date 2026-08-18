"""Subprocess-based git operations with token auth via http.extraHeader."""

from __future__ import annotations

import base64
import subprocess
from dataclasses import dataclass
from datetime import datetime

from prow_agent_eval.redact import redact_secrets, redact_url


@dataclass
class DiffResult:
    changed_files: list[str]
    full_diff: str


class Repo:
    def __init__(self, dir: str, token: str = "") -> None:
        self.dir = dir
        self.token = token

    @classmethod
    def clone(cls, url: str, dir: str, token: str = "") -> Repo:
        try:
            _git_run("", token, "clone", url, dir)
        except RuntimeError as err:
            raise RuntimeError(f"cloning {redact_url(url)}: {err}") from err
        return cls(dir, token)

    @classmethod
    def open_validated(cls, dir: str, token: str = "") -> Repo:
        repo = cls(dir, token)
        try:
            repo.rev_parse("--git-dir")
        except RuntimeError as err:
            raise RuntimeError(f"not a git repository {dir}: {err}") from err
        return repo

    def fetch(self, remote: str, *refs: str) -> None:
        _git_run(self.dir, self.token, "fetch", remote, *refs)

    def rev_parse(self, ref: str) -> str:
        out = _git_output(self.dir, self.token, "rev-parse", ref)
        return out.strip()

    def set_config(self, key: str, value: str) -> None:
        _git_run(self.dir, self.token, "config", key, value)

    def diff_against(self, base_sha: str) -> DiffResult:
        files_out = _git_output(self.dir, self.token, "diff", "--name-only", base_sha)
        full_diff = _git_output(self.dir, self.token, "diff", base_sha)
        return DiffResult(
            changed_files=_parse_name_only(files_out),
            full_diff=full_diff,
        )

    def diff_branches(self, branch1: str, branch2: str) -> DiffResult:
        range_spec = f"{branch1}...{branch2}"
        files_out = _git_output(
            self.dir, self.token, "diff", "--name-only", range_spec
        )
        full_diff = _git_output(self.dir, self.token, "diff", range_spec)
        return DiffResult(
            changed_files=_parse_name_only(files_out),
            full_diff=full_diff,
        )

    def current_branch(self) -> str:
        out = _git_output(self.dir, self.token, "rev-parse", "--abbrev-ref", "HEAD")
        return out.strip()

    def create_branch(self, name: str, start_ref: str) -> None:
        _git_run(self.dir, self.token, "checkout", "-b", name, start_ref)

    def checkout(self, ref: str) -> None:
        _git_run(self.dir, self.token, "checkout", ref)

    def push(self, remote: str, branch: str) -> None:
        _git_run(self.dir, self.token, "push", remote, branch)

    def delete_remote_branch(self, remote: str, branch: str) -> None:
        _git_run(self.dir, self.token, "push", remote, "--delete", branch)


def eval_branch_name(prefix: str) -> str:
    ts = datetime.now().strftime("%Y%m%d-%H%M%S")
    return f"{prefix}-eval-{ts}"


def short_sha(sha: str, n: int) -> str:
    if n <= 0 or not sha:
        return sha
    if len(sha) < n:
        return sha
    return sha[:n]


def _auth_header_args(token: str) -> list[str]:
    if not token:
        return []
    cred = base64.b64encode(f"x-access-token:{token}".encode()).decode()
    return ["-c", f"http.extraHeader=AUTHORIZATION: basic {cred}"]


def _git_run(dir: str, token: str, *args: str) -> None:
    cmd_args = _auth_header_args(token) + list(args)
    result = subprocess.run(
        ["git", *cmd_args],
        cwd=dir or None,
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode != 0:
        msg = result.stderr.strip()
        if msg:
            raise RuntimeError(redact_secrets(msg))
        raise RuntimeError(redact_secrets(str(result.returncode)))


def _git_output(dir: str, token: str, *args: str) -> str:
    cmd_args = _auth_header_args(token) + list(args)
    result = subprocess.run(
        ["git", *cmd_args],
        cwd=dir,
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode != 0:
        msg = result.stderr.strip()
        if msg:
            raise RuntimeError(redact_secrets(msg))
        raise RuntimeError(redact_secrets(str(result.returncode)))
    return result.stdout


def _parse_name_only(out: str) -> list[str]:
    return [line for line in out.strip().split("\n") if line]
