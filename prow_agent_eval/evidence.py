"""Evidence dataclasses and serialization for judge consumption."""

from __future__ import annotations

from dataclasses import asdict, dataclass, field


@dataclass
class MakeResult:
    collected: bool = False
    passed: bool = False
    output: str = ""
    error: str = ""


@dataclass
class BotReply:
    id: int = 0
    body: str = ""
    created_at: str = ""
    path: str = ""
    type: str = ""


@dataclass
class GitHubData:
    repo: str = ""
    base_branch: str = ""
    expected_branch: str = ""
    agent_branch: str = ""
    pr_number: int = 0
    pr_body: str = ""
    changed_files: list[str] = field(default_factory=list)
    expected_changed_files: list[str] = field(default_factory=list)
    full_diff: str = ""
    expected_full_diff: str = ""
    bot_replies: list[BotReply] = field(default_factory=list)
    posted_comments: dict = field(default_factory=dict)


@dataclass
class CaseEvidence:
    github: GitHubData = field(default_factory=GitHubData)
    annotations: dict = field(default_factory=dict)
    build_result: MakeResult = field(default_factory=MakeResult)
    test_result: MakeResult = field(default_factory=MakeResult)


def evidence_to_dict(ev: CaseEvidence) -> dict:
    return asdict(ev)
