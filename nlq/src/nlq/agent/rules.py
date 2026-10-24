"""Deterministic offline router: regex -> the *same* MCP tools."""

from __future__ import annotations

import json
import re
from typing import TYPE_CHECKING, Any

from .types import ToolCall

if TYPE_CHECKING:
    from .memory import Turn

_HELP = (
    "В упрощённом режиме я понимаю вопросы: что просрочено, что платить на этой "
    "неделе, сколько платим в рублях/долларах/евро, кто отвечает за что-то."
)

_CURRENCY_PATTERNS = (
    (re.compile(r"доллар|usd|\$"), "USD"),
    (re.compile(r"евро|eur|€"), "EUR"),
    (re.compile(r"руб|₽"), "RUB"),
)

_AMOUNT_CURRENCY = {"евро": "EUR", "eur": "EUR", "долл": "USD", "usd": "USD", "руб": "RUB"}

_DEARER = re.compile(r"(?:подписк|платеж).*(?:дороже|больше)\s+(\d+)\s*(евро|eur|долл|usd|руб)")
_WHO = re.compile(r"кто отвечает за\s+(.+)")


def _call(name: str, arguments: dict[str, Any]) -> ToolCall:
    return ToolCall(id=f"rules_{name}", name=name, arguments=json.dumps(arguments, ensure_ascii=False))


def _reply(reason: str, message: str) -> ToolCall:
    return _call("reply_without_data", {"reason": reason, "message": message})


def _detect_currency(text: str) -> str | None:
    for pattern, code in _CURRENCY_PATTERNS:
        if pattern.search(text):
            return code
    return None


def _detect_period(q: str) -> str | None:
    """Распознать именованный период в уточнении."""
    if "прошл" in q and "недел" in q:
        return "last_week"
    if "следующ" in q and "месяц" in q:
        return "next_month"
    if "прошл" in q and "месяц" in q:
        return "last_month"
    if "этом месяц" in q or "в этом месяце" in q:
        return "this_month"
    if "недел" in q:
        return "this_week"
    return None


def _refine(q: str, history: list[Turn]) -> list[ToolCall] | None:
    """Уточнение («а в долларах?», «а в прошлом месяце?»): повторить последний инструмент прошлого хода с переопределёнными параметрами."""
    last = None
    for turn in reversed(history):
        tools = turn.memory.get("tools") if turn.memory else None
        if isinstance(tools, list) and tools:
            cand = tools[-1]
            if isinstance(cand, dict) and cand.get("ok") and cand.get("name"):
                last = cand
                break
    if last is None:
        return None

    name = str(last["name"])
    if name not in ("find_occurrences", "sum_occurrences", "find_recurring_payments"):
        return None
    args: dict[str, Any] = dict(last.get("args", {}))

    changed = False
    currency = _detect_currency(q)
    if currency:
        args["currency"] = currency
        changed = True
    period = _detect_period(q)
    if period and name in ("find_occurrences", "sum_occurrences"):
        args["period"] = period
        args.pop("basis", None)
        changed = True
    if "оплачен" in q and "status" in args:
        args["status"] = "paid"
        changed = True
    elif "просроч" in q and "status" in args:
        args["status"] = "overdue"
        changed = True

    if not changed:
        return [_reply("needs_clarification", _HELP)]
    return [_call(name, args)]


def route(question: str, history: list[Turn] | None = None) -> list[ToolCall]:
    """Map a question to one tool call (occasionally a local reply)."""
    history = history or []
    q = question.lower().replace("ё", "е").strip()

    if (q.startswith("а ") or q.startswith("и ")) and history:
        refined = _refine(q, history)
        if refined is not None:
            return refined

    if "просроч" in q:
        return [_call("find_occurrences", {"status": "overdue"})]
    if "сегодня" in q:
        return [_call("find_occurrences", {"status": "unpaid", "period": "today"})]
    if "по месяц" in q or "динамик" in q or "как менялись" in q:
        mb_args: dict[str, Any] = {"status": "any", "months": 6}
        cur = _detect_currency(q)
        if cur:
            mb_args["currency"] = cur
        return [_call("monthly_breakdown", mb_args)]
    if "сравн" in q:
        cmp_args: dict[str, Any] = {
            "status": "any", "period_a": "last_month", "period_b": "this_month"
        }
        cur = _detect_currency(q)
        if cur:
            cmp_args["currency"] = cur
        return [_call("compare_periods", cmp_args)]
    if "нагрузк" in q or "у кого больше" in q:
        return [_call("workload", {"group_by": "assignee", "status": "unpaid"})]
    if "должност" in q or "персон" in q:
        return [_call("list_personas", {})]
    if "недел" in q:
        return [_call("find_occurrences", {"status": "upcoming", "period": "this_week"})]

    dearer = _DEARER.search(q)
    if dearer:
        amount = float(dearer.group(1))
        currency = _AMOUNT_CURRENCY.get(dearer.group(2), "EUR")
        return [
            _call(
                "find_recurring_payments",
                {"currency": currency, "min_amount": amount, "sort": "amount_desc"},
            )
        ]

    who = _WHO.search(q)
    if who:
        target = who.group(1).strip(" ?.!").strip()
        if target:
            return [_call("find_recurring_payments", {"title_query": target[:64]})]

    if "сколько" in q:
        currency = _detect_currency(q)
        args: dict[str, Any] = {"status": "any"}
        if "оплатил" in q or "оплачен" in q:
            args["status"] = "paid"
        if "в этом месяце" in q or "этом месяце" in q or "за этот месяц" in q:
            args["period"] = "this_month"
        elif "следующ" in q and "месяц" in q:
            args["period"] = "next_month"
        elif "в месяц" in q or "ежемесяч" in q:
            # "в месяц" without a named month -> average monthly load.
            args["basis"] = "run_rate"
        else:
            args["period"] = "this_month"
        if currency:
            args["currency"] = currency
        return [_call("sum_occurrences", args)]

    return [_reply("out_of_scope", _HELP)]
