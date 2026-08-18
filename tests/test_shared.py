"""Tests for shared-dir I/O."""

import os
import tempfile

import pytest

from prow_agent_eval.shared import (
    CaseMetadata,
    ensure_case_in_list,
    read_case_list,
    read_case_metadata,
    read_file,
    safe_join,
    write_case_list,
    write_case_metadata,
    write_file,
)


def test_write_read_case_metadata_round_trip():
    meta = CaseMetadata(
        case_name="case-001",
        pr_number=42,
        head_branch="eval-case-001-20260807-120000",
        base_branch="main",
        fixture_head_sha="abc123def456",
        jira_issue_key="TRT-1234",
        repo="openshift-trt/sippy-eval",
        bot_login="test-bot",
    )
    with tempfile.TemporaryDirectory() as directory:
        write_case_metadata(directory, meta)
        got = read_case_metadata(directory, meta.case_name)
    assert got == meta


def test_write_read_case_list():
    cases = ["case-001", "case-002", "case-003"]
    with tempfile.TemporaryDirectory() as directory:
        write_case_list(directory, cases)
        got = read_case_list(directory)
    assert got == cases


def test_write_read_file():
    with tempfile.TemporaryDirectory() as directory:
        write_file(directory, "test-file", "hello world")
        got = read_file(directory, "test-file")
    assert got == "hello world"


def test_write_case_metadata_clears_stale():
    first = CaseMetadata(
        case_name="case-001",
        pr_number=42,
        head_branch="eval-head",
        base_branch="main",
        fixture_head_sha="abc123",
        bot_login="bot",
        repo="o/r",
    )
    second = CaseMetadata(
        case_name="case-001",
        pr_number=0,
        head_branch="eval-head-2",
        base_branch="main",
        fixture_head_sha="def456",
        bot_login="",
        repo="o/r",
    )
    want = CaseMetadata(
        case_name="case-001",
        pr_number=0,
        head_branch="eval-head-2",
        base_branch="main",
        fixture_head_sha="def456",
        bot_login="",
        repo="o/r",
    )
    with tempfile.TemporaryDirectory() as directory:
        write_case_metadata(directory, first)
        write_case_metadata(directory, second)
        got = read_case_metadata(directory, want.case_name)
    assert got == want


def test_ensure_case_in_list_append_and_duplicate():
    with tempfile.TemporaryDirectory() as directory:
        write_case_list(directory, ["case-001", "case-002"])
        ensure_case_in_list(directory, "case-003")
        ensure_case_in_list(directory, "case-001")
        got = read_case_list(directory)
    assert got == ["case-001", "case-002", "case-003"]


def test_ensure_case_in_list_creates_when_missing():
    with tempfile.TemporaryDirectory() as directory:
        ensure_case_in_list(directory, "only")
        got = read_case_list(directory)
    assert got == ["only"]


def test_safe_join_rejects_traversal():
    with tempfile.TemporaryDirectory() as directory:
        with pytest.raises(ValueError, match="path escapes"):
            safe_join(directory, "../outside")
        with pytest.raises(ValueError, match="absolute path"):
            safe_join(directory, "/etc/passwd")
        with pytest.raises(ValueError, match="empty file name"):
            safe_join(directory, "")


def test_write_case_metadata_file_permissions():
    meta = CaseMetadata(case_name="case-001", base_branch="main")
    with tempfile.TemporaryDirectory() as directory:
        write_case_metadata(directory, meta)
        path = os.path.join(directory, "case-001.eval-base-branch")
        mode = os.stat(path).st_mode & 0o777
    assert mode == 0o600
