"""Evidence collection orchestration."""

from __future__ import annotations

import json
import logging
import os
import subprocess
from dataclasses import asdict
from pathlib import Path

from prow_agent_eval.cases import Case
from prow_agent_eval.evidence import BotReply, CaseEvidence, GitHubData, MakeResult
from prow_agent_eval.git import Repo
from prow_agent_eval.github import GitHubClient
from prow_agent_eval.shared import CaseMetadata

logger = logging.getLogger(__name__)

COLLECT_BY_MODE: dict[str, dict[str, bool]] = {
    "solve": {
        "build_result": True,
        "test_result": True,
        "expected_branch_diff": True,
        "bot_replies": False,
        "comment_map": False,
    },
    "followup": {
        "build_result": True,
        "test_result": True,
        "expected_branch_diff": False,
        "bot_replies": True,
        "comment_map": True,
    },
}


def collect(
    mode: str,
    case: Case,
    meta: CaseMetadata,
    client: GitHubClient,
    token: str,
    clone_dir: str,
    shared_dir: str,
) -> CaseEvidence:
    flags = COLLECT_BY_MODE.get(mode)
    if flags is None:
        raise ValueError(f"unknown mode {mode!r}")

    if flags["bot_replies"] and not meta.bot_login:
        raise ValueError("bot_replies collect requires bot login in metadata")
    if flags["expected_branch_diff"] and not case.input.expected_branch:
        raise ValueError("expected_branch_diff requires case input expected_branch")
    if flags["comment_map"] and not shared_dir:
        raise ValueError("comment_map collect requires shared dir")

    evidence = CaseEvidence(annotations=dict(case.annotations))
    repo = Repo.clone(client.clone_url(), clone_dir, token)

    head_branch, pr_number = _resolve_head(meta, client)
    repo.fetch("origin", head_branch)
    repo.checkout(f"origin/{head_branch}")

    fixture_sha = _resolve_fixture_sha(repo, meta)
    gh_data = _collect_github_data(
        flags, case, meta, client, repo, head_branch, fixture_sha, pr_number, shared_dir
    )
    evidence.github = gh_data

    if flags["build_result"]:
        evidence.build_result = _run_make(clone_dir, "build")
    if flags["test_result"]:
        evidence.test_result = _run_make(clone_dir, "test")

    return evidence


def write_evidence(case_dir: Path, evidence: CaseEvidence) -> None:
    case_dir.mkdir(parents=True, exist_ok=True)
    path = case_dir / "evidence.json"
    path.write_text(json.dumps(asdict(evidence), indent=2))
    os.chmod(path, 0o600)


def _resolve_head(meta: CaseMetadata, client: GitHubClient) -> tuple[str, int]:
    head_branch = meta.head_branch
    pr_number = meta.pr_number
    if not head_branch and pr_number > 0:
        pr = client.get_pr(pr_number)
        head_branch = pr.head.ref
        logger.info("discovered head branch from PR %s: %s", pr_number, head_branch)
    if not head_branch:
        raise ValueError(
            "no head branch available: set eval-head-branch, claude-branch, or pr-number in metadata"
        )
    return head_branch, pr_number


def _resolve_fixture_sha(repo: Repo, meta: CaseMetadata) -> str:
    if meta.fixture_head_sha:
        return meta.fixture_head_sha
    repo.fetch("origin", meta.base_branch)
    fixture_sha = repo.rev_parse(f"origin/{meta.base_branch}")
    logger.info("resolved fixture SHA from base branch %s: %s", meta.base_branch, fixture_sha[:8])
    return fixture_sha


def _collect_github_data(
    flags: dict[str, bool],
    case: Case,
    meta: CaseMetadata,
    client: GitHubClient,
    repo: Repo,
    head_branch: str,
    fixture_sha: str,
    pr_number: int,
    shared_dir: str,
) -> GitHubData:
    diff = repo.diff_against(fixture_sha)
    gh = GitHubData(
        agent_branch=head_branch,
        changed_files=diff.changed_files,
        full_diff=diff.full_diff,
        pr_number=pr_number,
    )

    if not gh.pr_number:
        pr = client.find_pr_by_head(head_branch)
        if pr is not None:
            gh.pr_number = pr.number
            logger.info("discovered agent-created PR %s", gh.pr_number)

    if gh.pr_number:
        pr = client.get_pr(gh.pr_number)
        gh.pr_body = pr.body or ""

    if flags["bot_replies"]:
        gh.bot_replies = _collect_bot_replies(client, gh.pr_number, meta.bot_login)

    if flags["comment_map"]:
        gh.posted_comments = _load_posted_comments(shared_dir, meta.case_name)

    if flags["expected_branch_diff"]:
        expected_branch = case.input.expected_branch
        repo.fetch("origin", expected_branch)
        expected_diff = repo.diff_branches(fixture_sha, f"origin/{expected_branch}")
        gh.expected_changed_files = expected_diff.changed_files
        gh.expected_full_diff = expected_diff.full_diff

    return gh


def _collect_bot_replies(
    client: GitHubClient, pr_number: int, bot_login: str
) -> list[BotReply]:
    if pr_number <= 0:
        raise ValueError("bot_replies collect requires a PR number")
    replies: list[BotReply] = []
    for c in client.list_issue_comments(pr_number):
        if c.user and c.user.login == bot_login:
            replies.append(
                BotReply(
                    id=c.id,
                    body=c.body or "",
                    created_at=c.created_at.isoformat() if c.created_at else "",
                    type="issue",
                )
            )
    for c in client.list_pr_review_comments(pr_number):
        if c.user and c.user.login == bot_login:
            replies.append(
                BotReply(
                    id=c.id,
                    body=c.body or "",
                    created_at=c.created_at.isoformat() if c.created_at else "",
                    path=c.path or "",
                    type="review",
                )
            )
    return replies


def _load_posted_comments(shared_dir: str, case_name: str) -> dict:
    map_path = os.path.join(shared_dir, f"{case_name}.comment-map.json")
    with open(map_path) as f:
        raw = json.load(f)
    out: dict = {}
    for key, val in raw.items():
        if isinstance(val, dict):
            out[key] = {
                "github_id": val.get("github_id", val.get("GitHubID", 0)),
                "category": val.get("category", val.get("Category", "")),
                "created_at": val.get("created_at", val.get("CreatedAt", "")),
            }
    return out


def _run_make(dir: str, target: str) -> MakeResult:
    result = subprocess.run(
        ["make", target],
        cwd=dir,
        capture_output=True,
        text=True,
        check=False,
    )
    output = result.stdout + result.stderr
    max_output = 10000
    if len(output) > max_output:
        output = output[-max_output:]
    make_result = MakeResult(
        collected=True,
        passed=result.returncode == 0,
        output=output.strip(),
    )
    if result.returncode != 0:
        make_result.error = f"exit code {result.returncode}"
    return make_result
