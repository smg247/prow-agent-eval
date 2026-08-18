"""Tests for secret redaction."""

from prow_agent_eval.redact import redact_secrets, redact_url


def test_redact_url_credentials():
    input_text = "fatal: cloning https://x-access-token:ghs_secret123@github.com/o/r.git failed"
    want = "fatal: cloning https://***:***@github.com/o/r.git failed"
    assert redact_url(input_text) == want
    assert redact_secrets(input_text) == want


def test_redact_basic_auth_header():
    assert redact_secrets("AUTHORIZATION: basic YWJjMTIz") == "AUTHORIZATION: basic ***"


def test_redact_plain_text_unchanged():
    text = "fatal: repository not found"
    assert redact_secrets(text) == text
