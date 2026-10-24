"""The eval set routes correctly offline for the rules-handled questions.

This runs the same comparison as ``eval/run_eval.py --mode rules`` in CI, so a
regression in the rules router or the question set is caught without a network
or a database.
"""

from __future__ import annotations

import json
from pathlib import Path

import pytest
import yaml

from nlq.agent import rules

QUESTIONS = Path(__file__).resolve().parents[2] / "eval" / "questions.yaml"


def _rules_questions() -> list[dict]:
    data = yaml.safe_load(QUESTIONS.read_text(encoding="utf-8"))
    return [q for q in data["questions"] if q["expect"].get("rules")]


def _args_match(expected: dict, actual: dict) -> bool:
    for key, want in expected.items():
        got = actual.get(key)
        if got is None:
            return False
        if isinstance(want, str) and isinstance(got, str):
            if want.lower() not in got.lower():
                return False
        elif isinstance(want, (int, float)) and isinstance(got, (int, float)):
            if float(want) != float(got):
                return False
        elif str(want) != str(got):
            return False
    return True


@pytest.mark.parametrize("q", _rules_questions(), ids=lambda q: f"q{q['id']}")
def test_rules_routes_expected_tool(q: dict) -> None:
    calls = [(c.name, json.loads(c.arguments)) for c in rules.route(q["q"])]
    expect = q["expect"]
    assert any(
        name == expect["tool"] and _args_match(expect.get("args", {}), args)
        for name, args in calls
    ), f"{q['q']} -> {calls}"
