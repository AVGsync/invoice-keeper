"""The agent loop and the top-level ``answer`` orchestrator (§7.5, §7.2).

``answer`` picks the brain (openrouter / rules / fake), runs the bounded loop,
and applies the two-level fallback: a manual backup-model retry inside the loop,
then the rules router if the LLM is unavailable. Numbers, blocks and the safe
fallback text are produced by ``render``/``envelope``; the model only proposes
tool calls and writes prose.
"""

from __future__ import annotations

import asyncio
import json
import logging
import time
from collections.abc import Awaitable, Callable
from datetime import date
from typing import Any, Protocol

from ..common.actor import Actor
from ..common.tool_specs import ALL_TOOLS, LOCAL_DATA_TOOLS, MCP_TOOLS
from . import envelope as env
from . import memory as mem
from . import rules
from .local_tools import REASON_FALLBACKS, validate_reply
from .mcp_client import McpError, McpSession
from .memory import Turn
from .openrouter import OpenRouterError
from .prompt import build_system_prompt
from .types import LLMResponse, LoopTelemetry, ToolCall, ToolResult

log = logging.getLogger("nlq.agent.loop")

StepHook = Callable[[str, dict[str, Any]], Awaitable[None]]

_TOOL_LABELS = {
    "find_occurrences": "Ищу платежи по срокам",
    "sum_occurrences": "Считаю суммы по валютам",
    "find_recurring_payments": "Ищу регулярные платежи",
    "find_people": "Ищу сотрудников",
    "get_payment_details": "Смотрю детали платежа",
    "monthly_breakdown": "Считаю по месяцам",
    "compare_periods": "Сравниваю периоды",
    "workload": "Считаю нагрузку",
    "list_personas": "Смотрю должности",
    "recall_conversation": "Вспоминаю прошлые ответы",
}


class LLMClient(Protocol):
    @property
    def primary(self) -> str: ...

    @property
    def backup(self) -> str | None: ...

    async def aclose(self) -> None: ...

    async def chat(
        self,
        messages: list[dict[str, Any]],
        tools: list[dict[str, Any]],
        tool_choice: str,
        *,
        timeout: float,
        model: str,
        fallback_models: list[str] | None = ...,
        user: str | None = ...,
    ) -> LLMResponse: ...


class LoopConfig:
    """Bounds and identity for one request (read off ``Settings``)."""

    def __init__(
        self,
        *,
        tools: list[dict[str, Any]],
        max_llm_calls: int,
        max_tool_calls_per_step: int,
        step_timeout: float,
        tool_timeout: float,
        total_timeout: float,
        mcp_url: str,
        rules_fallback: bool,
        tz: str,
    ) -> None:
        self.tools = tools
        self.max_llm_calls = max_llm_calls
        self.max_tool_calls_per_step = max_tool_calls_per_step
        self.step_timeout = step_timeout
        self.tool_timeout = tool_timeout
        self.total_timeout = total_timeout
        self.mcp_url = mcp_url
        self.rules_fallback = rules_fallback
        self.tz = tz


def _canonical(name: str, args: dict[str, Any]) -> str:
    return name + "|" + json.dumps(args, sort_keys=True, ensure_ascii=False)


class _Executor:
    """Runs tool calls against one MCP session with dedup + per-call timeout."""

    def __init__(
        self,
        session: McpSession | None,
        cfg: LoopConfig,
        actor: Actor,
        on_step: StepHook | None = None,
        history: list[Turn] | None = None,
    ) -> None:
        self._session = session
        self._cfg = cfg
        self._actor = actor
        self._on_step = on_step
        self._history = history or []
        self._step_n = 0
        self._seen: dict[str, ToolResult] = {}
        self.results: list[ToolResult] = []
        self.call_args: dict[str, dict[str, Any]] = {}

    async def run(self, call: ToolCall) -> ToolResult:
        try:
            raw_args = call.parsed_arguments()
        except (ValueError, json.JSONDecodeError):
            return self._fail(call, "bad_args", "аргументы не являются JSON")

        is_local_data = call.name in LOCAL_DATA_TOOLS
        model_cls, _ = (ALL_TOOLS if is_local_data else MCP_TOOLS).get(call.name, (None, None))
        if model_cls is None:
            return self._fail(call, "bad_args", "неизвестный инструмент")
        try:
            validated = model_cls.model_validate(raw_args)
        except Exception:  # pydantic ValidationError -> bad_args
            return self._fail(call, "bad_args", "неверные аргументы инструмента")

        args = validated.model_dump(exclude_none=True, mode="json")
        self.call_args[call.id] = args
        if self._on_step is not None:
            self._step_n += 1
            await self._on_step(
                "step",
                {"n": self._step_n, "tool": call.name, "label": _TOOL_LABELS.get(call.name, "Ищу данные")},
            )

        # Локальные data-инструменты исполняются без MCP (recall_conversation).
        if is_local_data:
            payload = {"ok": True, "tool": call.name, **mem.recall(
                self._history, args.get("query"), int(args.get("limit", 3))
            )}
            result = ToolResult(call_id=call.id, name=call.name, payload=payload)
            self.results.append(result)
            return result

        if self._session is None:
            return self._fail(call, "db_unavailable", "нет доступа к данным")
        key = _canonical(call.name, args)
        prior = self._seen.get(key)
        if prior is not None:
            repeated = ToolResult(
                call_id=call.id,
                name=call.name,
                payload={**prior.payload, "repeated": True},
                ms=0,
                repeated=True,
            )
            self.results.append(repeated)
            return repeated

        started = time.monotonic()
        try:
            payload = await asyncio.wait_for(
                self._session.call(call.name, args), timeout=self._cfg.tool_timeout
            )
        except TimeoutError:
            return self._fail(call, "timeout", "инструмент не ответил вовремя")
        except McpError:
            return self._fail(call, "db_unavailable", "нет доступа к данным")

        ms = int((time.monotonic() - started) * 1000)
        result = ToolResult(call_id=call.id, name=call.name, payload=payload, ms=ms)
        self._seen[key] = result
        self.results.append(result)
        return result

    def _fail(self, call: ToolCall, code: str, message: str) -> ToolResult:
        result = ToolResult(
            call_id=call.id,
            name=call.name,
            payload={"ok": False, "tool": call.name, "error": code, "message": message},
        )
        self.results.append(result)
        return result


# Keep at least this many seconds of the overall budget for the rules/template
# fallback after the loop, and never start an LLM attempt with less than this.
_FALLBACK_RESERVE = 2.5
_MIN_STEP_BUDGET = 1.0


async def _chat_with_fallback(
    llm: LLMClient,
    messages: list[dict[str, Any]],
    cfg: LoopConfig,
    tool_choice: str,
    deadline: float,
    user: str | None,
    telemetry: LoopTelemetry,
) -> LLMResponse:
    """Primary (with OpenRouter-side fallback list), then a manual backup try.
    Each attempt is capped at the smaller of the step timeout and the budget
    left before ``deadline``; the backup only runs if enough budget remains.
    """
    backup = llm.backup
    primary_timeout = min(cfg.step_timeout, max(deadline - time.monotonic(), 0.0))
    try:
        return await llm.chat(
            messages,
            cfg.tools,
            tool_choice,
            timeout=primary_timeout,
            model=llm.primary,
            fallback_models=[backup] if backup else None,
            user=user,
        )
    except OpenRouterError as exc:
        if exc.auth:
            log.error("openrouter_auth_or_credits: %s", exc)
            raise
        backup_timeout = min(cfg.step_timeout, max(deadline - time.monotonic(), 0.0))
        if backup is None or backup_timeout < _MIN_STEP_BUDGET:
            raise
        log.warning("primary failed (%s); trying backup %s", exc, backup)
        resp = await llm.chat(
            messages,
            cfg.tools,
            tool_choice,
            timeout=backup_timeout,
            model=backup,
            fallback_models=None,
            user=user,
        )
        telemetry.fallback = "backup"
        return resp


async def run_loop(
    *,
    question: str,
    actor: Actor,
    token: str,
    today: date,
    scope: str,
    mode: str,
    llm: LLMClient,
    cfg: LoopConfig,
    user: str | None,
    answer_id: str,
    history: list[Turn] | None = None,
    on_step: StepHook | None = None,
) -> dict[str, Any]:
    """Run the bounded agentic loop and return the finished envelope."""
    history = history or []
    start = time.monotonic()
    # Stop driving the LLM early enough to leave _FALLBACK_RESERVE for the
    # rules/template fallback within the overall budget (section 7.3).
    deadline = start + max(cfg.total_timeout - _FALLBACK_RESERVE, _MIN_STEP_BUDGET)
    telemetry = LoopTelemetry()
    messages: list[dict[str, Any]] = [
        {"role": "system", "content": build_system_prompt(actor, today, cfg.tz, scope)},
        *mem.history_messages(history),
        {"role": "user", "content": question},
    ]

    # Open the MCP session tolerantly: a greeting / out-of-scope answer
    # (reply_without_data) needs no data, so it should still work when MCP is
    # down. A data tool call with no session returns db_unavailable.
    session_cm = McpSession(cfg.mcp_url, token)
    try:
        session: McpSession | None = await session_cm.__aenter__()
    except McpError:
        session = None

    try:
        executor = _Executor(session, cfg, actor, on_step, history)
        for step in range(cfg.max_llm_calls):
            if deadline - time.monotonic() < _MIN_STEP_BUDGET:
                break
            last = step == cfg.max_llm_calls - 1
            choice = "required" if step == 0 else ("none" if last else "auto")
            resp = await _chat_with_fallback(
                llm,
                messages,
                cfg,
                choice,
                deadline,
                user,
                telemetry,
            )
            telemetry.add_usage(resp)
            messages.append(resp.raw_message or {"role": "assistant", "content": resp.content or ""})

            calls = resp.tool_calls
            if not calls:
                # Truncated output (finish_reason=length) may carry an ungrounded
                # half-number, so force the deterministic template instead.
                model_text = None if resp.finish_reason == "length" else resp.content
                return _finish(
                    answer_id, mode, model_text, question, executor, today, scope, telemetry, start, history
                )

            local = next((c for c in calls if c.name == "reply_without_data"), None)
            if local is not None:
                return _finish_local(answer_id, mode, local, today, scope, telemetry, start, history)

            await _execute_step(calls, executor, messages, cfg)

        return _finish(answer_id, mode, None, question, executor, today, scope, telemetry, start, history)
    finally:
        if session is not None:
            await session_cm.__aexit__(None, None, None)


async def _execute_step(
    calls: list[ToolCall],
    executor: _Executor,
    messages: list[dict[str, Any]],
    cfg: LoopConfig,
) -> None:
    runnable = calls[: cfg.max_tool_calls_per_step]
    overflow = calls[cfg.max_tool_calls_per_step :]
    outs = await asyncio.gather(*(executor.run(c) for c in runnable))
    for call, out in zip(runnable, outs, strict=True):
        messages.append(
            {"role": "tool", "tool_call_id": call.id, "content": out.for_llm()}
        )
    for call in overflow:
        messages.append(
            {
                "role": "tool",
                "tool_call_id": call.id,
                "content": json.dumps({"ok": False, "error": "too_many_calls"}),
            }
        )


def _finish(
    answer_id: str,
    mode: str,
    model_text: str | None,
    question: str,
    executor: _Executor,
    today: date,
    scope: str,
    telemetry: LoopTelemetry,
    start: float,
    history: list[Turn] | None = None,
) -> dict[str, Any]:
    history = history or []
    for res in executor.results:
        telemetry.record_tool(res, executor.call_args.get(res.call_id))
    return env.finalize_answer(
        answer_id=answer_id,
        mode=mode,
        model_text=model_text,
        question=question,
        results=executor.results,
        call_args=executor.call_args,
        today=today,
        scope=scope,
        telemetry=telemetry,
        latency_ms=int((time.monotonic() - start) * 1000),
        history_numbers=mem.history_numbers(history),
        memory_turns=sum(1 for t in history if t.in_prompt),
    )


def _finish_local(
    answer_id: str,
    mode: str,
    call: ToolCall,
    today: date,
    scope: str,
    telemetry: LoopTelemetry,
    start: float,
    history: list[Turn] | None = None,
) -> dict[str, Any]:
    history = history or []
    try:
        reply = validate_reply(call.parsed_arguments())
        message = reply.message
    except Exception:  # fall back to a canned message
        message = REASON_FALLBACKS["out_of_scope"]
    return env.finalize_answer(
        answer_id=answer_id,
        mode=mode,
        model_text=None,
        question="",
        results=[],
        call_args={},
        today=today,
        scope=scope,
        telemetry=telemetry,
        latency_ms=int((time.monotonic() - start) * 1000),
        local_message=message,
        memory_turns=sum(1 for t in history if t.in_prompt),
    )


async def run_rules(
    *,
    question: str,
    actor: Actor,
    token: str,
    today: date,
    scope: str,
    cfg: LoopConfig,
    answer_id: str,
    fallback_from: str | None = None,
    history: list[Turn] | None = None,
    on_step: StepHook | None = None,
) -> dict[str, Any]:
    """Deterministic router: regex -> the same MCP tools -> templated answer."""
    history = history or []
    start = time.monotonic()
    telemetry = LoopTelemetry()
    if fallback_from:
        telemetry.fallback = "rules"
    calls = rules.route(question, history)

    local = next((c for c in calls if c.name == "reply_without_data"), None)
    if local is not None:
        return _finish_local(answer_id, "rules", local, today, scope, telemetry, start, history)

    try:
        async with McpSession(cfg.mcp_url, token) as session:
            executor = _Executor(session, cfg, actor, on_step, history)
            for call in calls[: cfg.max_tool_calls_per_step]:
                await executor.run(call)
    except McpError:
        return env.error_envelope(
            answer_id=answer_id, mode="rules", error_code=env.ERR_DATA_UNAVAILABLE,
            text="Сейчас не могу получить данные о платежах. Попробуйте позже.",
            today=today, scope=scope, telemetry=telemetry, latency_ms=0,
        )
    env_out = _finish(answer_id, "rules", None, question, executor, today, scope, telemetry, start, history)
    env_out["notes"] = ["Упрощённый режим.", *env_out.get("notes", [])]
    return env_out


async def answer(
    *,
    question: str,
    actor: Actor,
    token: str,
    today: date,
    scope: str,
    llm_mode: str,
    llm: LLMClient | None,
    cfg: LoopConfig,
    user: str | None,
    answer_id: str,
    history: list[Turn] | None = None,
    on_step: StepHook | None = None,
) -> dict[str, Any]:
    """Top-level: run the chosen brain and apply the rules fallback (§7.2)."""
    history = history or []
    if llm_mode == "rules":
        return await run_rules(
            question=question, actor=actor, token=token, today=today, scope=scope,
            cfg=cfg, answer_id=answer_id, history=history, on_step=on_step,
        )

    assert llm is not None, "llm client required for openrouter/fake modes"
    mode = "fake" if llm_mode == "fake" else "llm"
    try:
        # Hard cap on the whole loop so a hung attempt cannot blow the Go budget;
        # the loop's own deadline normally fires first and leaves fallback time.
        async with asyncio.timeout(cfg.total_timeout):
            return await run_loop(
                question=question, actor=actor, token=token, today=today, scope=scope,
                mode=mode, llm=llm, cfg=cfg, user=user, answer_id=answer_id,
                history=history, on_step=on_step,
            )
    except TimeoutError:
        log.warning("agent loop exceeded %.1fs budget", cfg.total_timeout)
        if cfg.rules_fallback:
            return await run_rules(
                question=question, actor=actor, token=token, today=today, scope=scope,
                cfg=cfg, answer_id=answer_id, fallback_from=mode, history=history, on_step=on_step,
            )
        return env.error_envelope(
            answer_id=answer_id, mode=mode, error_code=env.ERR_TIMEOUT,
            text="Не успел ответить вовремя. Попробуйте ещё раз.",
            today=today, scope=scope, telemetry=LoopTelemetry(), latency_ms=0,
        )
    except McpError:
        # The MCP server itself is unreachable: no data path.
        return env.error_envelope(
            answer_id=answer_id, mode=mode, error_code=env.ERR_DATA_UNAVAILABLE,
            text="Сейчас не могу получить данные о платежах. Попробуйте позже.",
            today=today, scope=scope, telemetry=LoopTelemetry(),
            latency_ms=0,
        )
    except OpenRouterError:
        if cfg.rules_fallback:
            log.warning("LLM unavailable; falling back to rules")
            return await run_rules(
                question=question, actor=actor, token=token, today=today, scope=scope,
                cfg=cfg, answer_id=answer_id, fallback_from=mode, history=history, on_step=on_step,
            )
        return env.error_envelope(
            answer_id=answer_id, mode=mode, error_code=env.ERR_LLM_UNAVAILABLE,
            text="Ассистент сейчас недоступен. Попробуйте позже или команду /overdue.",
            today=today, scope=scope, telemetry=LoopTelemetry(), latency_ms=0,
        )
