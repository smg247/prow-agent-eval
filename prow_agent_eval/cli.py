"""Click CLI: init, judge, cleanup."""

from __future__ import annotations

import json
import logging
import os
import shutil
import sys
import tempfile
from pathlib import Path

import click
from agent_eval.config import EvalConfig

from prow_agent_eval.cases import Case, list_cases, load_case, load_init_repo
from prow_agent_eval.collect import collect, write_evidence
from prow_agent_eval.git import eval_branch_name, short_sha
from prow_agent_eval.github import GitHubClient, load_seeded_comments
from prow_agent_eval.scoring import run as run_scoring
from prow_agent_eval.shared import (
    CaseMetadata,
    ensure_case_in_list,
    read_case_list,
    read_case_metadata,
    write_case_list,
    write_case_metadata,
    write_file,
)

logger = logging.getLogger(__name__)


def _setup_logging() -> None:
    logging.basicConfig(
        level=logging.INFO,
        format="%(levelname)s %(message)s",
        stream=sys.stderr,
    )


def _resolve_token(token: str | None) -> str:
    return token or os.environ.get("GITHUB_TOKEN", "")


def _resolve_repo(cli_repo: str | None, config_path: str, case_repo: str = "") -> str:
    repo = cli_repo or load_init_repo(config_path) or case_repo
    if not repo:
        raise click.ClickException("no repo specified in --repo, config init.repo, or case input")
    return repo


@click.group()
def main() -> None:
    """Manage the eval lifecycle for agentic CI jobs in Prow."""


@main.command()
@click.option("--config", "config_path", required=True, type=click.Path(exists=True))
@click.option("--shared-dir", required=True, type=click.Path())
@click.option("--repo", default=None, help="owner/repo")
@click.option("--mode", type=click.Choice(["solve", "followup"]), required=True)
@click.option("--case", "case_name", default=None)
@click.option("--token", default=None)
@click.option(
    "--seed-token",
    default=None,
    help="Token used only to post seeded review comments. "
    "Falls back to GITHUB_SEED_TOKEN, then --token / GITHUB_TOKEN.",
)
def init(
    config_path: str,
    shared_dir: str,
    repo: str | None,
    mode: str,
    case_name: str | None,
    token: str | None,
    seed_token: str | None,
) -> None:
    """Create branches, PRs, and seed comments for eval cases."""
    _setup_logging()
    token = _resolve_token(token)
    seed_token = seed_token or os.environ.get("GITHUB_SEED_TOKEN") or token
    config = EvalConfig.from_yaml(config_path)
    config_dir = str(Path(config_path).parent)

    if case_name:
        case_names = [case_name]
        ensure_case_in_list(shared_dir, case_name)
    else:
        case_names = list_cases(config_dir, config.dataset.path)
        write_case_list(shared_dir, case_names)

    if not case_names:
        raise click.ClickException("no cases found")

    logger.info("initializing %d case(s) mode=%s", len(case_names), mode)
    for name in case_names:
        _init_case(config_path, config_dir, config, shared_dir, name, repo, mode, token, seed_token)
    logger.info("init complete count=%d", len(case_names))


def _init_case(
    config_path: str,
    config_dir: str,
    config: EvalConfig,
    shared_dir: str,
    case_name: str,
    cli_repo: str | None,
    mode: str,
    token: str,
    seed_token: str,
) -> None:
    case = load_case(config_dir, config.dataset.path, case_name)
    if not case.input.base_branch:
        raise click.ClickException(f"case {case_name}: input missing base_branch")

    repo_full = _resolve_repo(cli_repo, config_path, case.input.repo)
    logger.info("initializing case=%s repo=%s mode=%s", case_name, repo_full, mode)

    meta = CaseMetadata(
        case_name=case_name,
        base_branch=case.input.base_branch,
        jira_issue_key=case.input.jira_key,
        repo=repo_full,
    )
    _write_case_files(shared_dir, case)

    if mode == "solve" and not case.input.head_branch:
        write_case_metadata(shared_dir, meta)
        logger.info("case=%s done (metadata only)", case_name)
        return

    if not case.input.head_branch:
        raise click.ClickException(f"case {case_name}: input missing head_branch (required for followup)")

    client = GitHubClient(token, repo_full)
    clone_dir = tempfile.mkdtemp(prefix="prow-agent-eval-")
    try:
        _setup_eval_branch(case, meta, client, token, clone_dir, mode, shared_dir)
    finally:
        shutil.rmtree(clone_dir, ignore_errors=True)

    write_case_metadata(shared_dir, meta)

    if mode == "followup":
        seed_client = GitHubClient(seed_token, repo_full)
        _seed_case_comments(case, meta, seed_client, shared_dir)

    if meta.pr_number:
        logger.info(
            "case=%s done pr=%d branch=%s sha=%s",
            case_name,
            meta.pr_number,
            meta.head_branch,
            short_sha(meta.fixture_head_sha, 8),
        )
    else:
        logger.info(
            "case=%s done branch=%s sha=%s",
            case_name,
            meta.head_branch,
            short_sha(meta.fixture_head_sha, 8),
        )


def _setup_eval_branch(
    case: Case,
    meta: CaseMetadata,
    client: GitHubClient,
    token: str,
    clone_dir: str,
    mode: str,
    shared_dir: str,
) -> None:
    from prow_agent_eval.git import Repo

    repo = Repo.clone(client.clone_url(), clone_dir, token)
    repo.fetch("origin", case.input.head_branch)

    branch_prefix = case.name.replace("/", "-")
    eval_branch = eval_branch_name(branch_prefix)
    repo.create_branch(eval_branch, f"origin/{case.input.head_branch}")
    fixture_sha = repo.rev_parse("HEAD")
    repo.push("origin", eval_branch)

    bot_login = _read_bot_login(shared_dir)
    if not bot_login:
        try:
            bot_login = client.get_bot_login()
        except Exception as err:
            if mode == "followup":
                raise click.ClickException(f"getting bot login: {err}") from err
            logger.warning("could not get bot login case=%s: %s", case.name, err)

    meta.head_branch = eval_branch
    meta.fixture_head_sha = fixture_sha
    meta.bot_login = bot_login

    if mode != "followup":
        return

    pr_title = f"[eval] {case.name}"
    pr_body = f"Automated eval PR for case: {case.name}\nJira: {case.input.jira_key}"
    meta.pr_number = client.create_pr(eval_branch, case.input.base_branch, pr_title, pr_body)
    logger.info(
        "created PR case=%s pr=%d head=%s base=%s",
        case.name,
        meta.pr_number,
        eval_branch,
        case.input.base_branch,
    )


def _seed_case_comments(
    case: Case,
    meta: CaseMetadata,
    client: GitHubClient,
    shared_dir: str,
) -> None:
    comments_path = os.path.join(case.dir, "comments.json")
    if not os.path.isfile(comments_path):
        return
    comments = load_seeded_comments(comments_path)
    posted = client.seed_comments(meta.pr_number, comments)
    posted_dict = {
        k: {
            "github_id": v.github_id,
            "category": v.category,
            "created_at": v.created_at,
        }
        for k, v in posted.items()
    }
    write_file(shared_dir, f"{case.name}.comment-map.json", json.dumps(posted_dict))
    logger.info("seeded comments case=%s count=%d", case.name, len(comments))


def _write_case_files(shared_dir: str, case: Case) -> None:
    prefix = f"{case.name}."
    write_file(shared_dir, f"{prefix}eval-case", case.name)
    if case.input.expected_branch:
        write_file(shared_dir, f"{prefix}eval-expected-branch", case.input.expected_branch)
    jira_path = os.path.join(case.dir, "jira-issue.json")
    if os.path.isfile(jira_path):
        with open(jira_path) as f:
            write_file(shared_dir, f"{prefix}jira-issue.json", f.read())


def _read_bot_login(shared_dir: str) -> str:
    path = os.path.join(shared_dir, "gh-app-bot-login")
    if not os.path.isfile(path):
        return ""
    with open(path) as f:
        return f.read().strip()


@main.command()
@click.option("--config", "config_path", required=True, type=click.Path(exists=True))
@click.option("--shared-dir", required=True, type=click.Path())
@click.option("--artifact-dir", required=True, type=click.Path())
@click.option("--repo", default=None, help="owner/repo")
@click.option("--mode", type=click.Choice(["solve", "followup"]), required=True)
@click.option("--case", "case_name", default=None)
@click.option("--token", default=None)
def judge(
    config_path: str,
    shared_dir: str,
    artifact_dir: str,
    repo: str | None,
    mode: str,
    case_name: str | None,
    token: str | None,
) -> None:
    """Collect evidence, run judges, and emit reports."""
    _setup_logging()
    token = _resolve_token(token)
    config = EvalConfig.from_yaml(config_path)
    config_dir = str(Path(config_path).parent)

    if case_name:
        case_names = [case_name]
    else:
        case_names = read_case_list(shared_dir)

    if not case_names:
        raise click.ClickException("no cases found")

    os.makedirs(artifact_dir, exist_ok=True)
    logger.info("judging %d case(s)", len(case_names))

    run_root = Path(artifact_dir) / "eval" / "runs" / config.eval_name() / config.eval_name()
    cases_root = run_root / "cases"
    cases_root.mkdir(parents=True, exist_ok=True)

    case_dirs: list[Path] = []
    for name in case_names:
        case_dir = cases_root / name
        case_dir.mkdir(parents=True, exist_ok=True)
        try:
            _judge_case(
                config_path,
                config_dir,
                shared_dir,
                artifact_dir,
                name,
                repo,
                mode,
                token,
                case_dir,
            )
            case_dirs.append(case_dir)
        except Exception as err:
            logger.error("case %s error: %s", name, err)
            raise click.ClickException(f"case {name}: {err}") from err

    gate_ok = run_scoring(
        config,
        case_dirs,
        artifact_dir,
        run_id=config.eval_name(),
        shared_dir=shared_dir,
    )
    if not gate_ok:
        raise click.ClickException("eval gate failed: threshold regression(s) detected")
    logger.info("judge complete")


def _judge_case(
    config_path: str,
    config_dir: str,
    shared_dir: str,
    artifact_dir: str,
    case_name: str,
    cli_repo: str | None,
    mode: str,
    token: str,
    case_dir: Path,
) -> None:
    meta = read_case_metadata(shared_dir, case_name)
    case = load_case(config_dir, EvalConfig.from_yaml(config_path).dataset.path, case_name)
    repo_full = _resolve_repo(cli_repo, config_path, meta.repo)

    client = GitHubClient(token, repo_full)
    clone_dir = tempfile.mkdtemp(prefix="prow-agent-eval-judge-")
    try:
        logger.info("collecting data case=%s", case_name)
        evidence = collect(mode, case, meta, client, token, clone_dir, shared_dir)
        write_evidence(case_dir, evidence)
        _save_build_test_logs(artifact_dir, case_name, evidence)
    finally:
        shutil.rmtree(clone_dir, ignore_errors=True)


def _save_build_test_logs(artifact_dir: str, case_name: str, evidence) -> None:
    for suffix, field in [("build", "build_result"), ("test", "test_result")]:
        result = getattr(evidence, field)
        if not result.collected or not result.output:
            continue
        path = os.path.join(artifact_dir, f"{case_name}-{suffix}.log")
        with open(path, "w") as f:
            f.write(result.output)
        os.chmod(path, 0o600)


@main.command()
@click.option("--shared-dir", required=True, type=click.Path())
@click.option("--case", "case_name", default=None)
@click.option("--token", default=None)
def cleanup(shared_dir: str, case_name: str | None, token: str | None) -> None:
    """Close PRs and delete eval branches."""
    _setup_logging()
    token = _resolve_token(token)

    try:
        case_names = read_case_list(shared_dir)
    except Exception as err:
        logger.warning("could not read case list: %s", err)
        return

    if case_name:
        case_names = [case_name]

    logger.info("cleaning up %d case(s)", len(case_names))
    for name in case_names:
        _cleanup_case(shared_dir, name, token)
    logger.info("cleanup complete")


def _cleanup_case(shared_dir: str, case_name: str, token: str) -> None:
    try:
        meta = read_case_metadata(shared_dir, case_name)
    except Exception as err:
        logger.warning("could not read metadata case=%s: %s", case_name, err)
        return

    if not meta.repo:
        logger.warning("no repo in metadata, skipping case=%s", case_name)
        return

    try:
        client = GitHubClient(token, meta.repo)
    except Exception as err:
        logger.warning("could not create GitHub client case=%s: %s", case_name, err)
        return

    if meta.pr_number > 0:
        try:
            client.close_pr(meta.pr_number)
            logger.info("closed PR case=%s pr=%d", case_name, meta.pr_number)
        except Exception as err:
            logger.warning("could not close PR case=%s pr=%d: %s", case_name, meta.pr_number, err)

    if meta.head_branch:
        try:
            client.delete_branch(meta.head_branch)
            logger.info("deleted branch case=%s branch=%s", case_name, meta.head_branch)
        except Exception as err:
            logger.warning(
                "could not delete branch case=%s branch=%s: %s",
                case_name,
                meta.head_branch,
                err,
            )


if __name__ == "__main__":
    main()
