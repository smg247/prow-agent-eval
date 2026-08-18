"""Tests for JUnit XML generation."""

import xml.etree.ElementTree as ET

from prow_agent_eval.junit import write_junit


def test_write_junit_mixed_results(tmp_path):
    per_case = {
        "case-001": {
            "check_files": {"value": True, "rationale": "ok"},
            "no_secrets": {"value": False, "rationale": "Credential pattern found"},
        },
        "case-002": {
            "quality": {"value": None, "error": "Python error", "rationale": "Python error"},
        },
    }
    artifact_dir = str(tmp_path)
    write_junit(artifact_dir, "test-eval", per_case)

    path = tmp_path / "junit_test-eval.xml"
    data = path.read_text()
    assert "<?xml" in data

    root = ET.fromstring(data)
    suite = root.find("testsuite")
    assert suite is not None
    assert suite.get("tests") == "3"
    assert suite.get("failures") == "1"
    assert suite.get("errors") == "1"

    names = [tc.get("name") for tc in suite.findall("testcase")]
    assert "[test-eval] case-001 check_files" in names
    assert "[test-eval] case-001 no_secrets" in names
    assert "[test-eval] case-002 quality" in names
