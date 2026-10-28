#!/usr/bin/env python3
"""Contract tests for the dormant HLSL replay workflow template."""

from __future__ import annotations

import copy
import re
import unittest
from pathlib import Path

import yaml
from yaml.constructor import ConstructorError


HERE = Path(__file__).resolve().parent
WORKFLOW = HERE / "hlsl-scope-replay.yml"
RUN_BRANCH = "gpu-sci/hlsl-scope-replay-run-r1"
SOURCE_COMMIT = "6db920e72898abb20bf86c22652b401902e34598"
ACTION_PINS = {
    "actions/checkout": "fbc6f3992d24b796d5a048ff273f7fcc4a7b6c09",
    "actions/setup-go": "924ae3a1cded613372ab5595356fb5720e22ba16",
    "actions/upload-artifact": "ea165f8d65b6e75b540449e92b4886f43607fa02",
}
ARTIFACT_PATHS = [
    "main/workspace/artifacts/hlsl-scope-replay/regenerated-reading.json",
    "main/workspace/artifacts/hlsl-scope-replay/reading-diff.json",
]


class WorkflowLoader(yaml.SafeLoader):
    """YAML 1.2 booleans plus duplicate-key rejection.

    PyYAML's default YAML 1.1 resolver turns the GitHub key ``on`` into True.
    Copying the resolver table keeps this adjustment local to this loader.
    """


WorkflowLoader.yaml_implicit_resolvers = copy.deepcopy(
    yaml.SafeLoader.yaml_implicit_resolvers
)
for first_character, resolvers in WorkflowLoader.yaml_implicit_resolvers.items():
    WorkflowLoader.yaml_implicit_resolvers[first_character] = [
        (tag, pattern)
        for tag, pattern in resolvers
        if tag != "tag:yaml.org,2002:bool"
    ]
WorkflowLoader.add_implicit_resolver(
    "tag:yaml.org,2002:bool",
    re.compile(r"^(?:true|false)$", re.IGNORECASE),
    list("tTfF"),
)


def _construct_unique_mapping(loader, node, deep=False):
    loader.flatten_mapping(node)
    mapping = {}
    for key_node, value_node in node.value:
        key = loader.construct_object(key_node, deep=deep)
        try:
            duplicate = key in mapping
        except TypeError as error:
            raise ConstructorError(
                "while constructing a mapping",
                node.start_mark,
                "found an unhashable key",
                key_node.start_mark,
            ) from error
        if duplicate:
            raise ConstructorError(
                "while constructing a mapping",
                node.start_mark,
                f"found duplicate key {key!r}",
                key_node.start_mark,
            )
        mapping[key] = loader.construct_object(value_node, deep=deep)
    return mapping


WorkflowLoader.add_constructor(
    yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG,
    _construct_unique_mapping,
)


def parse_workflow(text: str) -> dict:
    document = yaml.load(text, Loader=WorkflowLoader)
    if not isinstance(document, dict):
        raise AssertionError("workflow must be a mapping")
    return document


def _all_mapping_keys(value):
    if isinstance(value, dict):
        for key, child in value.items():
            yield key
            yield from _all_mapping_keys(child)
    elif isinstance(value, list):
        for child in value:
            yield from _all_mapping_keys(child)


def validate_workflow(text: str) -> None:
    workflow = parse_workflow(text)

    triggers = workflow.get("on")
    assert isinstance(triggers, dict)
    assert set(triggers) == {"push", "workflow_dispatch"}
    assert triggers["push"] == {"branches": [RUN_BRANCH]}
    assert triggers["workflow_dispatch"] in (None, {})
    assert "pull_request" not in text
    assert "pull_request_target" not in text
    assert "secrets." not in text

    assert workflow.get("permissions") == {"contents": "read"}
    assert sum(key == "permissions" for key in _all_mapping_keys(workflow)) == 1

    jobs = workflow.get("jobs")
    assert isinstance(jobs, dict) and set(jobs) == {"replay"}
    replay = jobs["replay"]
    assert "permissions" not in replay
    assert replay.get("runs-on") == "ubuntu-24.04"
    assert replay.get("timeout-minutes") == 120
    assert "container" not in set(_all_mapping_keys(workflow))

    steps = replay.get("steps")
    assert isinstance(steps, list)
    action_steps = [step for step in steps if "uses" in step]
    assert len(action_steps) == 4
    action_counts = {action: 0 for action in ACTION_PINS}
    for step in action_steps:
        action, separator, pin = step["uses"].partition("@")
        assert separator == "@"
        assert re.fullmatch(r"[0-9a-f]{40}", pin)
        assert action in ACTION_PINS
        assert pin == ACTION_PINS[action]
        action_counts[action] += 1
    assert action_counts == {
        "actions/checkout": 2,
        "actions/setup-go": 1,
        "actions/upload-artifact": 1,
    }

    checkout_steps = [
        step for step in action_steps if step["uses"].startswith("actions/checkout@")
    ]
    controller, source = checkout_steps
    assert controller.get("with") == {
        "ref": "${{ github.sha }}",
        "path": "controller",
        "persist-credentials": False,
    }
    assert source.get("with") == {
        "ref": SOURCE_COMMIT,
        "path": "main",
        "persist-credentials": False,
    }

    upload = next(
        step
        for step in action_steps
        if step["uses"].startswith("actions/upload-artifact@")
    )
    upload_options = upload.get("with")
    assert upload_options.get("if-no-files-found") == "error"
    assert upload_options.get("path", "").splitlines() == ARTIFACT_PATHS


class WorkflowContractTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.workflow_text = WORKFLOW.read_text(encoding="utf-8")

    def test_workflow_contract(self):
        validate_workflow(self.workflow_text)

    def test_duplicate_critical_key_is_rejected(self):
        duplicate = self.workflow_text.replace(
            "permissions:\n  contents: read\n",
            "permissions:\n  contents: read\n  contents: write\n",
            1,
        )
        with self.assertRaises(ConstructorError):
            parse_workflow(duplicate)

    def test_security_and_execution_mutations_fail(self):
        mutations = {
            "pull request trigger": self.workflow_text.replace(
                "  workflow_dispatch:\n",
                "  workflow_dispatch:\n  pull_request:\n",
                1,
            ),
            "job write permissions": self.workflow_text.replace(
                "  replay:\n",
                "  replay:\n    permissions: write-all\n",
                1,
            ),
            "unpinned action": self.workflow_text.replace(
                "actions/checkout@fbc6f3992d24b796d5a048ff273f7fcc4a7b6c09",
                "actions/checkout@v5",
                1,
            ),
            "secret reference": self.workflow_text + "\n# secrets.REPLAY_TOKEN\n",
            "timeout": self.workflow_text.replace(
                "timeout-minutes: 120", "timeout-minutes: 121", 1
            ),
            "branch filter": self.workflow_text.replace(
                f"      - {RUN_BRANCH}\n", "      - main\n", 1
            ),
        }
        for name, mutation in mutations.items():
            with self.subTest(name=name), self.assertRaises(AssertionError):
                validate_workflow(mutation)


if __name__ == "__main__":
    unittest.main()
