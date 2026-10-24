"""Helpers for agent-loop tests: a fake MCP session and canned tool envelopes.

No network, no database, no real OpenRouter. The loop is driven by a scripted
FakeLLM and a FakeMcpSession that returns pre-baked envelopes, so every branch
(parallel calls, fallback, grounding, injection) is deterministic.
"""

from __future__ import annotations

from collections.abc import Callable
from datetime import date
from types import TracebackType
from typing import Any
from uuid import UUID

import pytest

from nlq.agent import loop as loop_mod
from nlq.agent.loop import LoopConfig
from nlq.common.actor import Actor
from nlq.common.tool_specs import LLM_TOOL_SCHEMAS

ACTOR = Actor(sub=UUID("11111111-1111-1111-1111-111111111111"), role="user", ch="web", rid="rid-1")
TODAY = date(2026, 10, 24)

EnvelopeFactory = Callable[[str, dict[str, Any]], dict[str, Any]]


class FakeMcpSession:
    """Implements the async-context + ``call`` surface the executor needs."""

    def __init__(self, responder: EnvelopeFactory, *, fail: bool = False) -> None:
        self._responder = responder
        self._fail = fail
        self.calls: list[tuple[str, dict[str, Any]]] = []

    async def __aenter__(self) -> FakeMcpSession:
        if self._fail:
            from nlq.agent.mcp_client import McpError

            raise McpError("boom")
        return self

    async def __aexit__(
        self,
        exc_type: type[BaseException] | None,
        exc: BaseException | None,
        tb: TracebackType | None,
    ) -> None:
        return None

    async def call(self, name: str, arguments: dict[str, Any]) -> dict[str, Any]:
        self.calls.append((name, arguments))
        return self._responder(name, arguments)


def install_mcp(monkeypatch: pytest.MonkeyPatch, session: FakeMcpSession) -> None:
    monkeypatch.setattr(loop_mod, "McpSession", lambda url, token: session)


def occurrences_envelope(rows: list[dict[str, Any]], total: int | None = None) -> dict[str, Any]:
    return {
        "ok": True,
        "tool": "find_occurrences",
        "today": "2026-10-24",
        "visibility": "team",
        "period": None,
        "total": total if total is not None else len(rows),
        "truncated": (total or len(rows)) > len(rows),
        "rows": rows,
    }


def overdue_rows() -> list[dict[str, Any]]:
    return [
        {
            "occurrence_id": "o1", "payment_id": "p1", "title": "Хостинг AWS",
            "amount": "120.00", "currency": "USD", "display": "120,00 $",
            "due_date": "2026-10-20", "days_overdue": 4, "is_paid": False,
            "assignee_id": "u-ivan", "assignee_name": "Иван Петров",
        },
        {
            "occurrence_id": "o2", "payment_id": "p2", "title": "Notion",
            "amount": "60.00", "currency": "EUR", "display": "60,00 €",
            "due_date": "2026-10-15", "days_overdue": 9, "is_paid": False,
            "assignee_id": "u-anna", "assignee_name": "Анна Смирнова",
        },
    ]


def make_cfg(**overrides: Any) -> LoopConfig:
    base: dict[str, Any] = {
        "tools": LLM_TOOL_SCHEMAS,
        "max_llm_calls": 4,
        "max_tool_calls_per_step": 3,
        "step_timeout": 10.0,
        "tool_timeout": 5.0,
        "total_timeout": 22.0,
        "mcp_url": "http://mcp/mcp",
        "rules_fallback": True,
        "tz": "Europe/Moscow",
    }
    base.update(overrides)
    return LoopConfig(**base)
