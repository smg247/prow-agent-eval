"""Case dataset loading from eval.yaml dataset paths."""

from __future__ import annotations

import os
from dataclasses import dataclass, field
from pathlib import Path

import yaml


@dataclass
class CaseInput:
    jira_key: str = ""
    base_branch: str = ""
    head_branch: str = ""
    expected_branch: str = ""
    repo: str = ""


@dataclass
class Case:
    name: str
    dir: str
    input: CaseInput = field(default_factory=CaseInput)
    annotations: dict = field(default_factory=dict)


def _dataset_dir(config_dir: str, dataset_path: str) -> str:
    if os.path.isabs(dataset_path):
        return dataset_path
    return os.path.join(config_dir, dataset_path)


def list_cases(config_dir: str, dataset_path: str) -> list[str]:
    cases_dir = _dataset_dir(config_dir, dataset_path)
    if not os.path.isdir(cases_dir):
        raise FileNotFoundError(f"listing cases in {cases_dir}")
    cases: list[str] = []
    for entry in sorted(os.listdir(cases_dir)):
        case_dir = os.path.join(cases_dir, entry)
        if os.path.isdir(case_dir) and os.path.isfile(os.path.join(case_dir, "input.yaml")):
            cases.append(entry)
    return cases


def load_case(config_dir: str, dataset_path: str, case_name: str) -> Case:
    case_dir = os.path.join(_dataset_dir(config_dir, dataset_path), case_name)
    input_path = os.path.join(case_dir, "input.yaml")
    with open(input_path) as f:
        raw = yaml.safe_load(f) or {}
    input_data = CaseInput(
        jira_key=raw.get("jira_key", ""),
        base_branch=raw.get("base_branch", ""),
        head_branch=raw.get("head_branch", ""),
        expected_branch=raw.get("expected_branch", ""),
        repo=raw.get("repo", ""),
    )
    annotations: dict = {}
    annotations_path = os.path.join(case_dir, "annotations.yaml")
    if os.path.isfile(annotations_path):
        with open(annotations_path) as f:
            annotations = yaml.safe_load(f) or {}
    return Case(name=case_name, dir=case_dir, input=input_data, annotations=annotations)


def load_init_repo(config_path: str) -> str:
    with open(config_path) as f:
        raw = yaml.safe_load(f) or {}
    init = raw.get("init") or {}
    return init.get("repo", "")
