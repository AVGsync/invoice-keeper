"""Shared dataclasses for the agent loop: tool calls, tool results, LLM output.

These keep the loop (``loop.py``), the LLM clients (``openrouter``/``rules``/
``fake``), the MCP client and the renderer talking in the same vocabulary
without importing each other.
"""

from __future__ import annotations

import json
from dataclasses import dataclass, field
from typing import Any


@dataclass(frozen=True, slots=True)
class ToolCall:
    """A single tool call requested by the model (or the rules router)."""

    id: str
    name: str
    arguments: str  # raw JSON string, exactly as the model produced it

    def parsed_arguments(self) -> dict[str, Any]:
        """Parse ``arguments`` leniently: '' -> {}; raise ValueError otherwise."""
        raw = self.arguments.strip()
        if not raw:
            return {}
        value = json.loads(raw)
        if not isinstance(value, dict):
            raise ValueError("tool arguments must be a JSON object")
        return value


@dataclass(slots=True)
class ToolResult:
    """Outcome of executing one tool call: the parsed MCP envelope + metadata.

    ``payload`` is the decoded JSON envelope (``{"ok": ..., ...}``). ``repeated``
    marks a deduplicated identical call (§7.5). ``ms`` is wall-clock latency.
    """

    call_id: str
    name: str
    payload: dict[str, Any]
    ms: int = 0
    repeated: bool = False

    @property
    def ok(self) -> bool:
        return bool(self.payload.get("ok"))

    @property
    def error(self) -> str | None:
        return None if self.ok else str(self.payload.get("error") or "error")

    def for_llm(self) -> str:
        """Compact JSON string handed back to the model as the tool message.

        Rows are already trimmed to <= 20 by the MCP server; here we only drop a
        few id-ish fields the model never needs to cite, keeping the context and
        the PII surface small (§7.6, §8.5).
        """
        return json.dumps(_slim_for_llm(self.payload), ensure_ascii=False)


@dataclass(slots=True)
class LLMResponse:
    """What one LLM step returned, normalised across providers."""

    content: str | None
    tool_calls: list[ToolCall]
    finish_reason: str | None
    model: str | None = None
    raw_message: dict[str, Any] | None = None  # appended verbatim to the history
    prompt_tokens: int = 0
    completion_tokens: int = 0
    cost_usd: float | None = None


@dataclass(slots=True)
class LoopTelemetry:
    """Accumulated over a request for ``meta`` and logging (§7.9, §11)."""

    llm_calls: int = 0
    model: str | None = None
    prompt_tokens: int = 0
    completion_tokens: int = 0
    cost_usd: float | None = None
    fallback: str = "none"  # none | backup | rules
    grounding: str = "ok"  # ok | failed
    tool_calls: list[dict[str, Any]] = field(default_factory=list)

    def add_usage(self, resp: LLMResponse) -> None:
        self.llm_calls += 1
        if resp.model:
            self.model = resp.model
        self.prompt_tokens += resp.prompt_tokens
        self.completion_tokens += resp.completion_tokens
        if resp.cost_usd is not None:
            self.cost_usd = (self.cost_usd or 0.0) + resp.cost_usd

    def record_tool(self, result: ToolResult, args: dict[str, Any] | None = None) -> None:
        rows = result.payload.get("rows")
        n = len(rows) if isinstance(rows, list) else 0
        entry: dict[str, Any] = {
            "name": result.name,
            "ok": result.ok,
            "rows": n,
            "ms": result.ms,
            # Validated args (post-pydantic), so eval can check key arguments and
            # the renderer/telemetry can show what was actually requested.
            "args": args or {},
        }
        if not result.ok:
            entry["error"] = result.error
        self.tool_calls.append(entry)


_SLIM_DROP = ("occurrence_id", "payment_id", "assignee_id", "user_id")


def _slim_for_llm(payload: dict[str, Any]) -> dict[str, Any]:
    out = dict(payload)
    rows = out.get("rows")
    if isinstance(rows, list):
        out["rows"] = [_slim_row(r) for r in rows if isinstance(r, dict)]
    return out


def _slim_row(row: dict[str, Any]) -> dict[str, Any]:
    slim = {k: v for k, v in row.items() if k not in _SLIM_DROP}
    # Keep the user_id the model needs for a follow-up assignee filter, but only
    # on people rows (find_people) where there are no amount fields.
    if "full_name" in row and "user_id" in row:
        slim["user_id"] = row["user_id"]
    title = slim.get("title")
    if isinstance(title, str) and len(title) > 80:
        slim["title"] = title[:80]
    return slim
