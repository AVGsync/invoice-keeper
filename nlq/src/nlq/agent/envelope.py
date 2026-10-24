"""Assemble the response envelope (§7.9).

The envelope is the one contract every client (web, Mini App, bot, Go) depends
on. It is built here from the model text (after cleaning + grounding) and the
code-built blocks/notes/followups, never directly from the model.
"""

from __future__ import annotations

from datetime import date
from typing import Any

from .grounding import build_allowed_numbers, clean_text, is_grounded
from .render import build_blocks, build_followups, build_notes, template_text
from .types import LoopTelemetry, ToolResult

# status / error_code closed sets (§7.9).
STATUS_OK = "ok"
STATUS_PARTIAL = "partial"
STATUS_ERROR = "error"

ERR_LLM_UNAVAILABLE = "llm_unavailable"
ERR_DATA_UNAVAILABLE = "data_unavailable"
ERR_TIMEOUT = "timeout"
ERR_BAD_QUESTION = "bad_question"


def finalize_answer(
    *,
    answer_id: str,
    mode: str,
    model_text: str | None,
    question: str,
    results: list[ToolResult],
    call_args: dict[str, dict[str, Any]],
    today: date,
    scope: str,
    telemetry: LoopTelemetry,
    latency_ms: int,
    local_message: str | None = None,
) -> dict[str, Any]:
    """Build the full envelope from tool results + (grounded) model text."""
    blocks = [] if local_message is not None else build_blocks(results)
    notes = (
        [] if local_message is not None else build_notes(results, call_args, question, scope)
    )
    followups = (
        _local_followups() if local_message is not None else build_followups(results)
    )

    data_error = _data_error(results, local_message)

    if local_message is not None:
        text = clean_text(local_message)
        # No digits allowed in a reply_without_data message (§5.7).
        if not is_grounded(text, {str(today.year), str(today.month), str(today.day)}):
            text = _strip_digits(text)
    else:
        allowed = build_allowed_numbers([r.payload for r in results if r.ok], question, today)
        text = clean_text(model_text)
        if not text or not is_grounded(text, allowed):
            text = template_text(blocks)
            telemetry.grounding = "failed"
            notes = [*notes, "Текст сформирован автоматически."]

    status, error_code = _status(data_error, blocks, local_message)

    envelope: dict[str, Any] = {
        "answer_id": answer_id,
        "status": status,
        "mode": mode,
        "text": text,
        "blocks": blocks,
        "followups": followups,
        "notes": notes,
        "meta": {
            "today": today.isoformat(),
            "visibility": scope,
            "model": telemetry.model,
            "llm_calls": telemetry.llm_calls,
            "tool_calls": telemetry.tool_calls,
            "tokens": {
                "prompt": telemetry.prompt_tokens,
                "completion": telemetry.completion_tokens,
            },
            "cost_usd": telemetry.cost_usd,
            "latency_ms": latency_ms,
        },
    }
    if error_code is not None:
        envelope["error_code"] = error_code
    return envelope


def error_envelope(
    *,
    answer_id: str,
    mode: str,
    error_code: str,
    text: str,
    today: date,
    scope: str,
    telemetry: LoopTelemetry,
    latency_ms: int,
) -> dict[str, Any]:
    return {
        "answer_id": answer_id,
        "status": STATUS_ERROR,
        "error_code": error_code,
        "mode": mode,
        "text": text,
        "blocks": [],
        "followups": [],
        "notes": [],
        "meta": {
            "today": today.isoformat(),
            "visibility": scope,
            "model": telemetry.model,
            "llm_calls": telemetry.llm_calls,
            "tool_calls": telemetry.tool_calls,
            "tokens": {"prompt": telemetry.prompt_tokens, "completion": telemetry.completion_tokens},
            "cost_usd": telemetry.cost_usd,
            "latency_ms": latency_ms,
        },
    }


def _data_error(results: list[ToolResult], local_message: str | None) -> str | None:
    if local_message is not None:
        return None
    for res in results:
        if not res.ok and res.error in ("db_unavailable", "source_unavailable"):
            return ERR_DATA_UNAVAILABLE
    return None


def _status(
    data_error: str | None, blocks: list[dict[str, Any]], local_message: str | None
) -> tuple[str, str | None]:
    if data_error is not None:
        return STATUS_ERROR, data_error
    if local_message is not None:
        return STATUS_OK, None
    return STATUS_OK, None


def _local_followups() -> list[str]:
    return ["Что просрочено?", "Сколько платим в этом месяце?"]


def _strip_digits(text: str) -> str:
    return "".join(ch for ch in text if not ch.isdigit()).strip() or (
        "Чем помочь по платежам?"
    )
