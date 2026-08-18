"""Shared-dir I/O for Prow inter-step metadata handoff."""

from __future__ import annotations

import os
from dataclasses import dataclass


@dataclass
class CaseMetadata:
    case_name: str
    pr_number: int = 0
    head_branch: str = ""
    base_branch: str = ""
    fixture_head_sha: str = ""
    jira_issue_key: str = ""
    repo: str = ""
    bot_login: str = ""


def safe_join(directory: str, name: str) -> str:
    if not name:
        raise ValueError("empty file name")
    if os.path.isabs(name):
        raise ValueError(f"absolute path not allowed: {name}")
    clean = os.path.normpath(name)
    if clean == ".." or clean.startswith(".." + os.sep):
        raise ValueError(f"path escapes directory: {name}")
    return os.path.join(directory, clean)


def write_case_metadata(directory: str, meta: CaseMetadata) -> None:
    prefix = f"{meta.case_name}."
    pr_val = str(meta.pr_number) if meta.pr_number else ""
    writes = {
        f"{prefix}pr-number": pr_val,
        f"{prefix}eval-head-branch": meta.head_branch,
        f"{prefix}eval-base-branch": meta.base_branch,
        f"{prefix}fixture-head-sha": meta.fixture_head_sha,
        f"{prefix}jira-issue-key": meta.jira_issue_key,
        f"{prefix}eval-repo": meta.repo,
        f"{prefix}bot-login": meta.bot_login,
    }
    for name, val in writes.items():
        path = safe_join(directory, name)
        if not val or (name.endswith("pr-number") and val == "0"):
            try:
                os.remove(path)
            except FileNotFoundError:
                pass
            continue
        with open(path, "w") as f:
            f.write(val)
        os.chmod(path, 0o600)


def read_case_metadata(directory: str, case_name: str) -> CaseMetadata:
    prefix = f"{case_name}."
    meta = CaseMetadata(case_name=case_name)

    meta.base_branch = _read_string(directory, f"{prefix}eval-base-branch")
    if not meta.base_branch:
        raise FileNotFoundError(
            f"required file {prefix}eval-base-branch not found in {directory}"
        )

    meta.head_branch = _read_string(directory, f"{prefix}eval-head-branch")
    if not meta.head_branch:
        meta.head_branch = _read_string(directory, f"{prefix}claude-branch")

    meta.fixture_head_sha = _read_string(directory, f"{prefix}fixture-head-sha")
    meta.jira_issue_key = _read_string(directory, f"{prefix}jira-issue-key")
    meta.repo = _read_string(directory, f"{prefix}eval-repo")
    meta.bot_login = _read_string(directory, f"{prefix}bot-login")

    pr_str = _read_string(directory, f"{prefix}pr-number")
    if pr_str:
        try:
            meta.pr_number = int(pr_str)
        except ValueError:
            pass

    return meta


def write_case_list(directory: str, cases: list[str]) -> None:
    content = "\n".join(cases) + "\n"
    path = os.path.join(directory, "eval-cases")
    with open(path, "w") as f:
        f.write(content)
    os.chmod(path, 0o600)


def ensure_case_in_list(directory: str, case_name: str) -> None:
    existing = read_case_list(directory)
    if case_name not in existing:
        write_case_list(directory, existing + [case_name])


def read_case_list(directory: str) -> list[str]:
    path = os.path.join(directory, "eval-cases")
    try:
        with open(path) as f:
            content = f.read().strip()
    except FileNotFoundError:
        return []
    return [line for line in content.split("\n") if line]


def write_file(directory: str, name: str, content: str) -> None:
    path = safe_join(directory, name)
    with open(path, "w") as f:
        f.write(content)
    os.chmod(path, 0o600)


def read_file(directory: str, name: str) -> str:
    return _read_string(directory, name)


def _read_string(directory: str, name: str) -> str:
    try:
        path = safe_join(directory, name)
    except ValueError:
        return ""
    try:
        with open(path) as f:
            return f.read().strip()
    except FileNotFoundError:
        return ""
