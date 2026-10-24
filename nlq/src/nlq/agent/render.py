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


_CUR_ORDER = ("RUB", "USD", "EUR")


def _fmt_month(value: str | None) -> str:
    if not value:
        return ""
    try:
        y, m = value.split("-")
        return f"{int(m):02d}.{int(y)}"
    except (ValueError, AttributeError):
        return value


def _ordered_currencies(seen: list[str]) -> list[str]:
    known = [c for c in _CUR_ORDER if c in seen]
    extra = [c for c in seen if c not in _CUR_ORDER]
    return known + extra


def _monthly_breakdown_block(payload: dict[str, Any]) -> dict[str, Any]:
    """Pivot the per-(month,currency) rows into a month × currency table."""
    rows = payload.get("rows", [])
    months: list[str] = []
    currencies: list[str] = []
    cell: dict[tuple[str, str], str] = {}
    for r in rows:
        mo = r.get("month", "")
        cur = r.get("currency", "")
        if mo not in months:
            months.append(mo)
        if cur not in currencies:
            currencies.append(cur)
        cell[(mo, cur)] = r.get("display", "")
    order = _ordered_currencies(currencies)
    columns = [{"key": "month", "label": "Месяц"}]
    columns += [{"key": cur, "label": cur, "align": "end"} for cur in order]
    table_rows: list[dict[str, Any]] = []
    for mo in months:
        row: dict[str, Any] = {"month": _fmt_month(mo)}
        for cur in order:
            row[cur] = cell.get((mo, cur)) or None
        table_rows.append(row)
    return {
        "type": "table",
        "kind": "monthly_breakdown",
        "title": "Суммы по месяцам",
        "columns": columns,
        "rows": table_rows,
        "coverage_until": payload.get("coverage_until"),
        "coverage_incomplete": bool(payload.get("coverage_incomplete", False)),
    }


def _compare_periods_block(payload: dict[str, Any]) -> dict[str, Any]:
    by = payload.get("by_currency", {})
    columns = [
        {"key": "currency", "label": "Валюта"},
        {"key": "a", "label": "Было (A)", "align": "end"},
        {"key": "b", "label": "Стало (B)", "align": "end"},
        {"key": "delta", "label": "Изменение", "align": "end"},
    ]
    table_rows: list[dict[str, Any]] = []
    for cur in _ordered_currencies(list(by.keys())):
        v = by.get(cur) or {}
        table_rows.append(
            {
                "currency": cur,
                "a": v.get("a_display"),
                "b": v.get("b_display"),
                "delta": v.get("delta_display"),
            }
        )
    return {
        "type": "table",
        "kind": "compare_periods",
        "title": "Сравнение периодов",
        "columns": columns,
        "rows": table_rows,
        "a": payload.get("a"),
        "b": payload.get("b"),
    }


def _workload_block(payload: dict[str, Any]) -> dict[str, Any]:
    columns = [
        {"key": "name", "label": "Кто"},
        {"key": "currency", "label": "Валюта"},
        {"key": "count", "label": "Платежей", "align": "end"},
        {"key": "total", "label": "Сумма", "align": "end"},
    ]
    table_rows: list[dict[str, Any]] = []
    for r in payload.get("rows", []):
        table_rows.append(
            {
                "name": r.get("name"),
                "currency": r.get("currency"),
                "count": str(r.get("count", 0)),
                "total": r.get("display"),
            }
        )
    return {
        "type": "table",
        "kind": "workload",
        "title": "Нагрузка по платежам",
        "columns": columns,
        "rows": table_rows,
        "truncated": bool(payload.get("groups_truncated", False)),
    }


def _list_personas_block(payload: dict[str, Any]) -> dict[str, Any]:
    columns = [
        {"key": "persona", "label": "Должность"},
        {"key": "users_count", "label": "Сотрудников", "align": "end"},
        {"key": "active_payments", "label": "Активных платежей", "align": "end"},
    ]
    table_rows: list[dict[str, Any]] = []
    for r in payload.get("rows", []):
        ap = r.get("active_payments")
        table_rows.append(
            {
                "persona": r.get("persona"),
                "users_count": str(r.get("users_count", 0)),
                "active_payments": None if ap is None else str(ap),
            }
        )
    return {
        "type": "table",
        "kind": "personas",
        "title": "Должности",
        "columns": columns,
        "rows": table_rows,
        "total": int(payload.get("total", len(table_rows))),
    }


_BUILDERS = {
    "find_occurrences": _occurrence_block,
    "sum_occurrences": _currency_totals_block,
    "find_recurring_payments": _payment_list_block,
    "find_people": _people_list_block,
    "get_payment_details": _payment_list_block,
    "monthly_breakdown": _monthly_breakdown_block,
    "compare_periods": _compare_periods_block,
    "workload": _workload_block,
    "list_personas": _list_personas_block,
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
    "get_payment_details": [
        "Когда ближайший срок оплаты?",
        "Кто за это отвечает?",
    ],
    "monthly_breakdown": [
        "Сравни с прошлым месяцем",
        "Что просрочено?",
    ],
    "compare_periods": [
        "Покажи по месяцам",
        "Что просрочено?",
    ],
    "workload": [
        "У кого больше всего просрочено?",
        "Сколько платим в этом месяце?",
    ],
    "list_personas": [
        "Какая нагрузка по должностям?",
        "Кто отвечает за аренду?",
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
    if kind == "table":
        n = len(block.get("rows", []))
        title = block.get("title", "Данные")
        if n == 0:
            return f"{title}: данных не найдено."
        return f"{title}: подробности в таблице ниже."
    return ""
