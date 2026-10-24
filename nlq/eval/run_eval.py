"""Eval runner for the NL-assistant demo questions (§12.4).

    uv run python eval/run_eval.py --mode rules
    uv run python eval/run_eval.py --mode http --runs 3   # against a running agent

Modes:
  rules  offline: check that the deterministic router picks the expected tool and
         key arguments for the questions it is designed to handle (no DB needed).
  http   drive a *running* nlq-agent (NLQ_AGENT_URL) with signed actor tokens and
         check the tools it actually called + basic envelope invariants. Set the
         agent's NLQ_LLM=openrouter and OPENROUTER_API_KEY to eval a real model;
         NLQ_LLM=fake/rules eval the offline brains end-to-end.

Admission criterion (§12.4): 100% on questions 1-8 and >= 90% overall, per model,
across the requested number of runs.
"""

from __future__ import annotations

import argparse
import json
import os
import sys
import time
import uuid
from pathlib import Path
from typing import Any

import yaml

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from nlq.agent import rules

QUESTIONS = Path(__file__).with_name("questions.yaml")


def load_questions() -> dict[str, Any]:
    return yaml.safe_load(QUESTIONS.read_text(encoding="utf-8"))


def _args_match(expected: dict[str, Any], actual: dict[str, Any]) -> bool:
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


def _match(expected: dict[str, Any], calls: list[tuple[str, dict[str, Any]]]) -> bool:
    want_tool = expected["tool"]
    want_args = expected.get("args", {})
    return any(name == want_tool and _args_match(want_args, args) for name, args in calls)


# --- rules mode (offline) ---


def _run_rules(data: dict[str, Any]) -> tuple[int, int]:
    passed = total = 0
    for q in data["questions"]:
        expect = q["expect"]
        if not expect.get("rules"):
            continue
        total += 1
        calls = [(c.name, json.loads(c.arguments)) for c in rules.route(q["q"])]
        ok = _match(expect, calls)
        passed += int(ok)
        print(f"[{'ok ' if ok else 'FAIL'}] #{q['id']:>2} {q['q']}")
        if not ok:
            print(f"        expected {expect['tool']}{expect.get('args', {})}, got {calls}")
    return passed, total


# --- http mode (drive a running agent) ---


def _sign(sub: str, role: str, seed: str) -> str:
    sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))
    from nlq.devtoken import load_private_key, sign_actor

    return sign_actor(load_private_key(seed), sub=sub, role=role, ch="web", rid=str(uuid.uuid4()), ttl_seconds=120)


# Seed user UUIDs (tests/fixtures/seed.sql): used so "мои"/"у меня" and
# per-person questions resolve to a real person with payments.
_SEED_ADMIN = "11110000-0000-0000-0000-000000000001"  # Анна (admin)
_SEED_USER = "11110000-0000-0000-0000-000000000002"   # Иван (user)


def _run_http(data: dict[str, Any], runs: int, require_mode: str | None, model: str) -> tuple[int, int]:
    import httpx

    base = os.environ.get("NLQ_AGENT_URL", "http://127.0.0.1:8090")
    seed = os.environ.get("NLQ_ACTOR_PRIVATE_KEY")
    if not seed:
        print("set NLQ_ACTOR_PRIVATE_KEY (base64 Ed25519 seed) for http mode", file=sys.stderr)
        sys.exit(2)
    sub_admin = os.environ.get("EVAL_ADMIN_SUB", _SEED_ADMIN)
    sub_user = os.environ.get("EVAL_USER_SUB", _SEED_USER)
    want_models = {m.strip() for m in model.split(",") if m.strip()}

    passed = total = 0
    for _ in range(runs):
        for q in data["questions"]:
            total += 1
            role = q["role"].split()[0]
            sub = sub_admin if role == "admin" else sub_user
            token = _sign(sub, role, seed)
            try:
                resp = httpx.post(
                    f"{base}/v1/ask",
                    headers={"Authorization": f"Bearer {token}"},
                    json={"question": q["q"], "channel": "web"},
                    timeout=30,
                )
                env = resp.json()
            except Exception as exc:
                print(f"[FAIL] #{q['id']:>2} transport: {type(exc).__name__}")
                continue
            calls = [
                (c.get("name"), c.get("args", {}))
                for c in env.get("meta", {}).get("tool_calls", [])
            ]
            names = [n for n, _ in calls]
            want = q["expect"]["tool"]
            reasons: list[str] = []
            # The tool and its key args must be present (reply_without_data has no calls).
            if want == "reply_without_data":
                if names:
                    reasons.append("expected no MCP calls")
            elif not _match(q["expect"], calls):
                reasons.append(f"missing {want}{q['expect'].get('args', {})}")
            # The answer must have succeeded with real data.
            if env.get("status") != "ok":
                reasons.append(f"status={env.get('status')}/{env.get('error_code')}")
            # No tool returned an error envelope.
            if any(not c.get("ok", True) for c in env.get("meta", {}).get("tool_calls", [])):
                reasons.append("a tool returned an error")
            # When evaluating a real model, the answer must come from the LLM,
            # not the rules fallback, and from one of the evaluated models.
            if require_mode and env.get("mode") != require_mode:
                reasons.append(f"mode={env.get('mode')} != {require_mode}")
            if want_models and require_mode == "llm":
                m = (env.get("meta") or {}).get("model")
                if m not in want_models:
                    reasons.append(f"model={m} not in evaluated set")
            ok = not reasons
            passed += int(ok)
            flag = "ok " if ok else "FAIL"
            why = "" if ok else "  <- " + "; ".join(reasons)
            print(
                f"[{flag}] #{q['id']:>2} {q['q']} -> tools={names} "
                f"mode={env.get('mode')}{why}"
            )
    return passed, total


def main() -> None:
    parser = argparse.ArgumentParser(prog="run_eval")
    parser.add_argument("--mode", choices=["rules", "http"], default="rules")
    parser.add_argument("--runs", type=int, default=1)
    parser.add_argument("--model", default=os.environ.get("NLQ_MODELS", ""), help="evaluated model id(s)")
    parser.add_argument(
        "--require-mode",
        choices=["llm", "fake", "rules"],
        default=None,
        help="require this answer mode (use 'llm' when selecting a model)",
    )
    args = parser.parse_args()

    data = load_questions()
    started = time.time()
    if args.mode == "rules":
        passed, total = _run_rules(data)
    else:
        passed, total = _run_http(data, args.runs, args.require_mode, args.model)

    pct = (100.0 * passed / total) if total else 0.0
    print(f"\n{args.mode}: {passed}/{total} = {pct:.0f}%  ({time.time() - started:.1f}s)  model={args.model}")
    sys.exit(0 if passed == total else 1)


if __name__ == "__main__":
    main()
