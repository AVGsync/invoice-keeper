"""Deterministic offline router (§7.8): regex -> the *same* MCP tools.

Used as ``NLQ_LLM=rules`` (demo without internet) and as the automatic fallback
when the LLM is unavailable. It emits the same ToolCall objects the loop would
get from the model, so execution, rendering and grounding are identical; the
envelope just carries ``mode:"rules"``.
"""

from __future__ import annotations

import json
import re
from typing import Any

from .types import ToolCall

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


def route(question: str) -> list[ToolCall]:
    """Map a question to one tool call (occasionally a local reply)."""
    q = question.lower().replace("ё", "е").strip()

    if "просроч" in q:
        return [_call("find_occurrences", {"status": "overdue"})]
    if "сегодня" in q:
        return [_call("find_occurrences", {"status": "unpaid", "period": "today"})]
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
            # "в месяц" without a named month -> average monthly load (decision #2).
            args["basis"] = "run_rate"
        else:
            args["period"] = "this_month"
        if currency:
            args["currency"] = currency
        return [_call("sum_occurrences", args)]

    return [_reply("out_of_scope", _HELP)]
