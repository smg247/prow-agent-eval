"""Secret redaction for logs and error messages."""

import re

_URL_CRED_PATTERN = re.compile(r"://[^/@\s]+:[^/@\s]+@")
_BASIC_AUTH_PATTERN = re.compile(r"(?i)(AUTHORIZATION:\s*basic\s+)[A-Za-z0-9+/=]+")


def redact_url(url: str) -> str:
    return _URL_CRED_PATTERN.sub("://***:***@", url)


def redact_secrets(text: str) -> str:
    text = _URL_CRED_PATTERN.sub("://***:***@", text)
    text = _BASIC_AUTH_PATTERN.sub(r"\1***", text)
    return text
