"""Build the answer blocks, notes and follow-ups from tool results (§7.7, §7.9).

The model only writes prose; everything structured (tables, totals, the safe
fallback text) is assembled here from the tool envelopes, so a compromised model
cannot fabricate a number or a row.
"""

from __future__ import annotations

from datetime import date
from typing import Any

from .types import ToolResult

MAX_BLOCKS = 3

_CURRENCY_WORD = {"RUB": "рублях", "USD": "долларах", "EUR": "евро"}


def _fmt_date(iso: str | None) -> str | None:
    if not iso:
        return None
    try:
        return date.fromisoformat(iso[:10]).strftime("%d.%m.%Y")
    except ValueError:
        return iso


def _occurrence_status(item: dict[str, Any]) -> str:
    if item.get("is_paid"):
        return "paid"
    if item.get("days_overdue"):
        return "overdue"
    return "upcoming"


def _occurrence_block(payload: dict[str, Any]) -> dict[str, Any]:
    items: list[dict[str, Any]] = []
    for r in payload.get("rows", []):
        items.append(
            {
                "occurrence_id": r.get("occurrence_id", ""),
                "payment_id": r.get("payment_id", ""),
                "title": r.get("title", ""),
                "amount": r.get("amount", "0"),
                "currency": r.get("currency"),
                "display": r.get("display", ""),
                "due_date": r.get("due_date", ""),
                "status": _occurrence_status(r),
                "days_overdue": r.get("days_overdue") or 0,
                "assignee_id": r.get("assignee_id"),
                "assignee_name": r.get("assignee_name"),
            }
        )
    return {
        "type": "occurrence_list",
        "title": "Платежи",
        "total": int(payload.get("total", len(items))),
        "truncated": bool(payload.get("truncated", False)),
        "coverage_until": payload.get("coverage_until"),
        "coverage_incomplete": bool(payload.get("coverage_incomplete", False)),
        "items": items,
    }


def _currency_totals_block(payload: dict[str, Any]) -> dict[str, Any]:
    return {
        "type": "currency_totals",
        "title": "Суммы по валютам",
        "basis": payload.get("basis", "period_sum"),
        "period": payload.get("period"),
        "coverage_until": payload.get("coverage_until"),
        "coverage_incomplete": bool(payload.get("coverage_incomplete", False)),
        "by_currency": payload.get("by_currency", {}),
        "groups": payload.get("groups", []),
        "groups_truncated": bool(payload.get("groups_truncated", False)),
    }


def _payment_list_block(payload: dict[str, Any]) -> dict[str, Any]:
    items: list[dict[str, Any]] = []
    for r in payload.get("rows", []):
        items.append(
            {
                "payment_id": r.get("payment_id", ""),
                "title": r.get("title", ""),
                "amount": r.get("amount", "0"),
                "currency": r.get("currency"),
                "display": r.get("display", ""),
                "assignee_name": r.get("assignee_name"),
                "assignee_persona": r.get("assignee_persona"),
                "next_due": r.get("next_due"),
                "is_ended": bool(r.get("is_ended", False)),
            }
        )
    return {
        "type": "payment_list",
        "title": "Регулярные платежи",
        "total": int(payload.get("total", len(items))),
        "truncated": bool(payload.get("truncated", False)),
        "items": items,
    }


def _people_list_block(payload: dict[str, Any]) -> dict[str, Any]:
    items: list[dict[str, Any]] = []
    for r in payload.get("rows", []):
        items.append(
            {
                "user_id": r.get("user_id", ""),
                "full_name": r.get("full_name", ""),
                "role": r.get("role"),
                "persona": r.get("persona"),
                "active_payments": r.get("active_payments"),
            }
        )
    return {"type": "people_list", "title": "Сотрудники", "items": items}


_BUILDERS = {
    "find_occurrences": _occurrence_block,
    "sum_occurrences": _currency_totals_block,
    "find_recurring_payments": _payment_list_block,
    "find_people": _people_list_block,
}


def build_blocks(results: list[ToolResult]) -> list[dict[str, Any]]:
    """Last successful result per tool, max 3 blocks; drop a people lookup that
    was only used to feed a later assignee filter (§7.7 p.1)."""
    last_ok: dict[str, ToolResult] = {}
    last_index: dict[str, int] = {}
    for i, res in enumerate(results):
        if res.ok and res.name in _BUILDERS:
            last_ok[res.name] = res
            last_index[res.name] = i

    people_followed = "find_people" in last_index and any(
        name in last_index and last_index[name] > last_index["find_people"]
        for name in ("find_occurrences", "find_recurring_payments")
    )

    ordered = sorted(last_ok.values(), key=lambda r: last_index[r.name])
    blocks: list[dict[str, Any]] = []
    for res in ordered:
        if res.name == "find_people" and people_followed:
            continue
        blocks.append(_BUILDERS[res.name](res.payload))
    return blocks[:MAX_BLOCKS]


def build_notes(
    results: list[ToolResult], call_args: dict[str, dict[str, Any]], question: str, scope: str
) -> list[str]:
    notes: list[str] = []
    q = question.lower()
    has_currency_filter = any(a.get("currency") for a in call_args.values())
    category_requested = any(a.get("category") for a in call_args.values())
    for res in results:
        if not res.ok:
            continue
        p = res.payload
        if p.get("truncated"):
            total = p.get("total")
            shown = len(p.get("rows", []))
            notes.append(f"Показаны {shown} из {total}.")
        if p.get("groups_truncated"):
            notes.append("Показаны не все группы по ответственным.")
        if p.get("coverage_incomplete"):
            until = _fmt_date(p.get("coverage_until"))
            if until:
                notes.append(f"Периоды после {until} ещё не созданы; будущие суммы могут быть неполными.")
            else:
                notes.append("Будущие периоды ещё не созданы; суммы могут быть неполными.")
        # Decision #2: always say what was computed for a sum.
        if res.name == "sum_occurrences":
            if p.get("basis") == "run_rate":
                notes.append(
                    "Это средняя месячная нагрузка по активным платежам "
                    "(ежемесячный ×1, квартальный ÷3, полугодовой ÷6, годовой ÷12)."
                )
            else:
                period = p.get("period")
                if period and period.get("from") and period.get("to"):
                    notes.append(
                        f"Посчитаны начисления со сроком {_fmt_date(period['from'])}"
                        f"–{_fmt_date(period['to'])} (это не средняя месячная нагрузка)."
                    )
                else:
                    notes.append("Посчитаны все найденные начисления (период не задан).")
        # Decision #3: categories exist but B's column may be absent.
        if p.get("category_unavailable"):
            notes.append("Категории платежей пока нет в системе — фильтр по категории не применялся.")
    if has_currency_filter:
        notes.append("Другие валюты не учитывались и не пересчитывались.")
    if (
        "подписк" in q
        and not category_requested
        and any(r.name == "find_recurring_payments" and r.ok for r in results)
    ):
        notes.append("Показаны все подходящие платежи; уточните категорию, если нужна конкретная.")
    if scope == "own":
        notes.append("Показаны только ваши платежи.")
    # De-duplicate, keep order.
    seen: set[str] = set()
    unique: list[str] = []
    for n in notes:
        if n not in seen:
            seen.add(n)
            unique.append(n)
    return unique


_FOLLOWUPS: dict[str, list[str]] = {
    "find_occurrences": [
        "Кто отвечает за эти платежи?",
        "Что платить на этой неделе?",
    ],
    "sum_occurrences": [
        "Что из этого уже оплачено?",
        "Что просрочено?",
    ],
    "find_recurring_payments": [
        "Когда ближайший срок оплаты?",
        "Что просрочено?",
    ],
    "find_people": [
        "За что они отвечают?",
        "Что просрочено?",
    ],
}


def build_followups(results: list[ToolResult]) -> list[str]:
    for res in reversed(results):
        if res.ok and res.name in _FOLLOWUPS:
            return _FOLLOWUPS[res.name][:3]
    return ["Что просрочено?", "Сколько платим в этом месяце?"]


def template_text(blocks: list[dict[str, Any]]) -> str:
    """Deterministic plain-text summary, used when the model text is missing or
    fails grounding (§4.3, §7.7 p.3)."""
    if not blocks:
        return "Не удалось получить данные. Попробуйте переформулировать вопрос."
    parts: list[str] = []
    for block in blocks:
        parts.append(_template_for_block(block))
    return " ".join(p for p in parts if p).strip()


def _template_for_block(block: dict[str, Any]) -> str:
    kind = block.get("type")
    if kind == "occurrence_list":
        total = block.get("total", 0)
        if total == 0:
            if block.get("coverage_incomplete"):
                until = _fmt_date(block.get("coverage_until"))
                if until:
                    return f"Подходящих платежей не найдено; периоды после {until} ещё не созданы."
            return "Подходящих платежей не найдено."
        return f"Найдено платежей: {total}. Подробности в таблице ниже."
    if kind == "payment_list":
        total = block.get("total", 0)
        if total == 0:
            return "Регулярных платежей не найдено."
        return f"Найдено регулярных платежей: {total}. Подробности в таблице ниже."
    if kind == "people_list":
        n = len(block.get("items", []))
        if n == 0:
            return "Сотрудников не найдено."
        names = ", ".join(i.get("full_name", "") for i in block.get("items", [])[:5])
        return f"Найдено: {names}."
    if kind == "currency_totals":
        by = block.get("by_currency", {})
        if not by:
            return "Платежей за период не найдено."
        # Падеж «в рублях/долларах/евро» читается верно во всех трёх шаблонах
        # ниже («нагрузка в долларах …», «суммы … в долларах …»), тогда как без
        # предлога получалось «нагрузка — долларах».
        chunks = [f"в {_CURRENCY_WORD.get(cur, cur)}: {v.get('display', '')}" for cur, v in by.items()]
        body = "; ".join(chunks)
        if block.get("basis") == "run_rate":
            return f"Средняя месячная нагрузка — {body}."
        period = block.get("period")
        if period and period.get("from") and period.get("to"):
            return (
                f"Суммы со сроком {_fmt_date(period['from'])}–{_fmt_date(period['to'])} — {body}."
            )
        return "Суммы по валютам — " + body + "."
    return ""
