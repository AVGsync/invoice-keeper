"""compare_periods: сравнение двух периодов по каждой валюте."""

from __future__ import annotations

from decimal import Decimal
from typing import Any

from ...common import money
from ...common.actor import Actor
from ...common.dates import resolve_period
from ...common.errors import SourceUnavailable
from ...common.tool_specs import ComparePeriodsArgs
from ..db import load_sql
from ..fragments import STATUS_SQL, escape_like
from ..policy import Visibility, resolve_assignee
from ..state import AppState
from ._common import period_block, render_occurrences_base


async def _totals(
    state: AppState,
    status: str,
    bounds: tuple[Any, Any] | None,
    args: ComparePeriodsArgs,
    assignee_model: Any,
    visibility: Visibility,
) -> dict[str, dict[str, Any]]:
    title = args.title_query
    sql = load_sql("sum_occurrences.sql").format(
        base=render_occurrences_base(state), status_sql=STATUS_SQL[status]
    )
    rows = await state.db.ro_query(
        sql,
        state.today(),                          # $1
        bounds[0] if bounds else None,          # $2
        bounds[1] if bounds else None,          # $3
        args.currency,                          # $4
        title,                                  # $5 tsquery
        escape_like(title) if title else None,  # $6 ilike
        assignee_model,                         # $7
        visibility.own_filter,                  # $8
        None,                                   # $9 category (не используется)
    )
    return {
        r["currency"]: {"total": Decimal(r["total"]), "count": int(r["cnt"])}
        for r in rows
    }


async def execute(
    args: ComparePeriodsArgs, state: AppState, actor: Actor, visibility: Visibility
) -> dict[str, Any]:
    if not state.schema.payments_available:
        raise SourceUnavailable("payment data is not available yet")

    today = state.today()
    assignee_model = resolve_assignee(args.assignee_id, actor, visibility)
    bounds_a = resolve_period(args.period_a, today, args.a_from, args.a_to)
    bounds_b = resolve_period(args.period_b, today, args.b_from, args.b_to)

    a = await _totals(state, args.status, bounds_a, args, assignee_model, visibility)
    b = await _totals(state, args.status, bounds_b, args, assignee_model, visibility)

    by_currency: dict[str, Any] = {}
    for cur in sorted(set(a) | set(b)):
        a_total = a.get(cur, {}).get("total", Decimal(0))
        b_total = b.get(cur, {}).get("total", Decimal(0))
        delta = b_total - a_total
        delta_pct = (
            float(round((delta / a_total) * 100, 1)) if a_total != 0 else None
        )
        by_currency[cur] = {
            "a_total": money.to_amount_string(a_total),
            "b_total": money.to_amount_string(b_total),
            "delta": money.to_amount_string(delta),
            "delta_pct": delta_pct,
            "a_count": a.get(cur, {}).get("count", 0),
            "b_count": b.get(cur, {}).get("count", 0),
            "a_display": money.display(a_total, cur),
            "b_display": money.display(b_total, cur),
            "delta_display": _signed_display(delta, cur),
        }

    return {
        "today": today.isoformat(),
        "visibility": visibility.scope,
        "a": period_block(bounds_a),
        "b": period_block(bounds_b),
        "by_currency": by_currency,
    }


def _signed_display(value: Decimal, currency: str) -> str:
    """Сумма со знаком «+»/«−» перед модулем (минус уже даёт display)."""
    if value > 0:
        return "+" + money.display(value, currency)
    return money.display(value, currency)
