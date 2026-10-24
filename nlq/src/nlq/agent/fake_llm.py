"""Scripted LLM for tests and the e2e smoke (§12.2).

``NLQ_LLM=fake`` replays a fixed sequence of assistant turns (tool calls or final
content) so the loop, MCP calls, rendering and grounding can be exercised with no
network and no cost. Tests pass a script in memory; at runtime a JSON file named
by ``NLQ_FAKE_SCRIPT`` is loaded.
"""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from .types import LLMResponse, ToolCall


class FakeLLM:
    """Drop-in replacement for OpenRouterClient with the same ``chat`` shape."""

    def __init__(self, script: list[dict[str, Any]], model: str = "fake/model") -> None:
        self._script = list(script)
        self._step = 0
        self._model = model
        # Копии messages каждого вызова chat — для тестов истории (SPEC §D.6).
        self.received: list[list[dict[str, Any]]] = []

    @property
    def primary(self) -> str:
        return self._model

    @property
    def backup(self) -> str | None:
        return None

    async def aclose(self) -> None:  # parity with OpenRouterClient
        return None

    async def chat(
        self,
        messages: list[dict[str, Any]],
        tools: list[dict[str, Any]],
        tool_choice: str,
        *,
        timeout: float,
        model: str,
        fallback_models: list[str] | None = None,
        user: str | None = None,
    ) -> LLMResponse:
        self.received.append([dict(m) for m in messages])
        if self._step >= len(self._script):
            return LLMResponse(content="", tool_calls=[], finish_reason="stop", model=self._model)
        step = self._script[self._step]
        self._step += 1
        return _build_response(step, self._model)


def _build_response(step: dict[str, Any], model: str) -> LLMResponse:
    raw_calls = step.get("tool_calls") or []
    calls: list[ToolCall] = []
    raw_history_calls: list[dict[str, Any]] = []
    for i, rc in enumerate(raw_calls):
        args = rc.get("arguments", {})
        args_str = args if isinstance(args, str) else json.dumps(args, ensure_ascii=False)
        call_id = str(rc.get("id") or f"fake_{i}")
        calls.append(ToolCall(id=call_id, name=rc["name"], arguments=args_str))
        raw_history_calls.append(
            {
                "id": call_id,
                "type": "function",
                "function": {"name": rc["name"], "arguments": args_str},
            }
        )
    content = step.get("content")
    raw_message: dict[str, Any] = {"role": "assistant", "content": content or ""}
    if raw_history_calls:
        raw_message["tool_calls"] = raw_history_calls
    return LLMResponse(
        content=content if isinstance(content, str) else None,
        tool_calls=calls,
        finish_reason=step.get("finish_reason", "tool_calls" if calls else "stop"),
        model=model,
        raw_message=raw_message,
        prompt_tokens=int(step.get("prompt_tokens", 0)),
        completion_tokens=int(step.get("completion_tokens", 0)),
    )


def load_script(path: str) -> list[dict[str, Any]]:
    data = json.loads(Path(path).read_text(encoding="utf-8"))
    if not isinstance(data, list):
        raise ValueError("fake script must be a JSON list of steps")
    return data
