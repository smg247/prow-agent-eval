"""Tests for evidence dataclasses."""

from prow_agent_eval.evidence import (
    BotReply,
    CaseEvidence,
    GitHubData,
    MakeResult,
    evidence_to_dict,
)


def test_evidence_to_dict_round_trip_fields():
    ev = CaseEvidence(
        github=GitHubData(
            agent_branch="feature",
            pr_number=7,
            pr_body="fix bug",
            changed_files=["a.go"],
            expected_changed_files=["a.go", "b.go"],
            full_diff="diff",
            expected_full_diff="expected",
            bot_replies=[BotReply(id=1, body="hi", type="issue")],
            posted_comments={"c1": {"body": "note"}},
        ),
        annotations={"scope_creep": False},
        build_result=MakeResult(collected=True, passed=True, output="ok"),
        test_result=MakeResult(collected=True, passed=False, output="fail", error="err"),
    )
    d = evidence_to_dict(ev)
    assert d["github"]["agent_branch"] == "feature"
    assert d["github"]["pr_number"] == 7
    assert d["github"]["bot_replies"][0]["body"] == "hi"
    assert d["annotations"]["scope_creep"] is False
    assert d["build_result"]["passed"] is True
    assert d["test_result"]["error"] == "err"
