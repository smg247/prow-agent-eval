"""PyGithub wrapper for PR, comment, and branch operations."""

from __future__ import annotations

import json
import logging
import os
from dataclasses import dataclass
from typing import Any

from github import Auth, Github, GithubException
from tenacity import retry, retry_if_exception_type, stop_after_attempt, wait_exponential

logger = logging.getLogger(__name__)

RETRYABLE = (GithubException, ConnectionError, TimeoutError)


@dataclass
class SeededComment:
    body: str
    category: str = ""
    path: str = ""
    line: int = 0
    side: str = ""


@dataclass
class PostedComment:
    github_id: int = 0
    category: str = ""
    created_at: str = ""


class GitHubClient:
    def __init__(
        self,
        token: str,
        repo_full_name: str,
        api_url: str | None = None,
        clone_url: str | None = None,
    ) -> None:
        if not token:
            token = os.environ.get("GITHUB_TOKEN", "")
        if not token:
            raise ValueError("GITHUB_TOKEN not set and --token not provided")
        parts = repo_full_name.split("/", 1)
        if len(parts) != 2:
            raise ValueError(f"invalid repo format {repo_full_name!r}, expected owner/repo")
        self._owner, self._repo = parts
        self._clone_url = clone_url
        base = api_url or "https://api.github.com"
        self._gh = Github(auth=Auth.Token(token), base_url=base)

    @classmethod
    def from_token(cls, token: str, repo_full_name: str) -> GitHubClient:
        return cls(token, repo_full_name)

    def owner(self) -> str:
        return self._owner

    def repo(self) -> str:
        return self._repo

    def set_clone_url(self, url: str) -> None:
        self._clone_url = url

    def clone_url(self) -> str:
        if self._clone_url:
            return self._clone_url
        return f"https://github.com/{self._owner}/{self._repo}.git"

    @retry(
        retry=retry_if_exception_type(RETRYABLE),
        stop=stop_after_attempt(3),
        wait=wait_exponential(multiplier=1, min=1, max=10),
        reraise=True,
    )
    def get_bot_login(self) -> str:
        return self._gh.get_user().login

    @retry(
        retry=retry_if_exception_type(RETRYABLE),
        stop=stop_after_attempt(3),
        wait=wait_exponential(multiplier=1, min=1, max=10),
        reraise=True,
    )
    def create_pr(self, head: str, base: str, title: str, body: str) -> int:
        repo = self._gh.get_repo(f"{self._owner}/{self._repo}")
        pr = repo.create_pull(title=title, body=body, head=head, base=base)
        return pr.number

    @retry(
        retry=retry_if_exception_type(RETRYABLE),
        stop=stop_after_attempt(3),
        wait=wait_exponential(multiplier=1, min=1, max=10),
        reraise=True,
    )
    def close_pr(self, number: int) -> None:
        repo = self._gh.get_repo(f"{self._owner}/{self._repo}")
        pr = repo.get_pull(number)
        pr.edit(state="closed")

    @retry(
        retry=retry_if_exception_type(RETRYABLE),
        stop=stop_after_attempt(3),
        wait=wait_exponential(multiplier=1, min=1, max=10),
        reraise=True,
    )
    def get_pr(self, number: int) -> Any:
        repo = self._gh.get_repo(f"{self._owner}/{self._repo}")
        return repo.get_pull(number)

    @retry(
        retry=retry_if_exception_type(RETRYABLE),
        stop=stop_after_attempt(3),
        wait=wait_exponential(multiplier=1, min=1, max=10),
        reraise=True,
    )
    def find_pr_by_head(self, head_branch: str) -> Any | None:
        repo = self._gh.get_repo(f"{self._owner}/{self._repo}")
        pulls = repo.get_pulls(head=f"{self._owner}:{head_branch}", state="all")
        for pr in pulls:
            return pr
        return None

    @retry(
        retry=retry_if_exception_type(RETRYABLE),
        stop=stop_after_attempt(3),
        wait=wait_exponential(multiplier=1, min=1, max=10),
        reraise=True,
    )
    def delete_branch(self, branch: str) -> None:
        repo = self._gh.get_repo(f"{self._owner}/{self._repo}")
        ref = repo.get_git_ref(f"heads/{branch}")
        ref.delete()

    @retry(
        retry=retry_if_exception_type(RETRYABLE),
        stop=stop_after_attempt(3),
        wait=wait_exponential(multiplier=1, min=1, max=10),
        reraise=True,
    )
    def post_issue_comment(self, number: int, body: str) -> Any:
        repo = self._gh.get_repo(f"{self._owner}/{self._repo}")
        return repo.get_issue(number).create_comment(body)

    @retry(
        retry=retry_if_exception_type(RETRYABLE),
        stop=stop_after_attempt(3),
        wait=wait_exponential(multiplier=1, min=1, max=10),
        reraise=True,
    )
    def post_pr_review_comment(
        self, number: int, body: str, path: str, line: int, side: str = ""
    ) -> Any:
        repo = self._gh.get_repo(f"{self._owner}/{self._repo}")
        pr = repo.get_pull(number)
        return pr.create_review_comment(body, pr.head.sha, path, line)

    def seed_comments(
        self, pr_number: int, comments: list[SeededComment]
    ) -> dict[str, PostedComment]:
        posted: dict[str, PostedComment] = {}
        for i, sc in enumerate(comments):
            key = f"comment-{i + 1:03d}"
            if sc.path and sc.line > 0:
                rc = self.post_pr_review_comment(pr_number, sc.body, sc.path, sc.line, sc.side)
                posted[key] = PostedComment(
                    github_id=rc.id,
                    category=sc.category,
                    created_at=rc.created_at.isoformat() if rc.created_at else "",
                )
            else:
                ic = self.post_issue_comment(pr_number, sc.body)
                posted[key] = PostedComment(
                    github_id=ic.id,
                    category=sc.category,
                    created_at=ic.created_at.isoformat() if ic.created_at else "",
                )
        return posted

    @retry(
        retry=retry_if_exception_type(RETRYABLE),
        stop=stop_after_attempt(3),
        wait=wait_exponential(multiplier=1, min=1, max=10),
        reraise=True,
    )
    def list_issue_comments(self, number: int) -> list[Any]:
        repo = self._gh.get_repo(f"{self._owner}/{self._repo}")
        return list(repo.get_issue(number).get_comments())

    @retry(
        retry=retry_if_exception_type(RETRYABLE),
        stop=stop_after_attempt(3),
        wait=wait_exponential(multiplier=1, min=1, max=10),
        reraise=True,
    )
    def list_pr_review_comments(self, number: int) -> list[Any]:
        repo = self._gh.get_repo(f"{self._owner}/{self._repo}")
        return list(repo.get_pull(number).get_review_comments())


def load_seeded_comments(path: str) -> list[SeededComment]:
    with open(path) as f:
        raw = json.load(f)
    comments: list[SeededComment] = []
    for item in raw:
        comments.append(
            SeededComment(
                body=item.get("body", ""),
                category=item.get("category", ""),
                path=item.get("path", ""),
                line=int(item.get("line", 0)),
                side=item.get("side", ""),
            )
        )
    return comments
