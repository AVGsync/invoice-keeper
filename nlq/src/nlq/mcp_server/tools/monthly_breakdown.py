"""monthly_breakdown: суммы по месяцам и валютам — динамика расходов."""

from __future__ import annotations

import calendar
from datetime import date
from decimal import Decimal
from typing import Any

from ...common import money
from ...common.actor import Actor
from ...common.dates import parse_date
from ...common.errors import BadArgs, SourceUnavailable
from ...common.tool_specs import MonthlyBreakdownArgs
from ..db import load_sql
from ..fragments import STATUS_SQL, escape_like
from ..policy import Visibility, resolve_assignee
from ..state import AppState
from ._common import compute_coverage, render_occurrences_base

_MAX_MONTHS = 24


def _first_of_month(d: date) -> date:
    return date(d.year, d.month, 1)


def _last_of_month(d: date) -> date:
    return date(d.year, d.month, calendar.monthrange(d.year, d.month)[1])


def _shift_months(d: date, delta: int) -> date:
    """Первый день месяца, сдвинутого на ``delta`` месяцев от месяца ``d``."""
    index = (d.year * 12 + (d.month - 1)) + delta
    return date(index // 12, index % 12 + 1, 1)


def _month_span(a: date, b: date) -> int:
    return (b.year * 12 + b.month) - (a.year * 12 + a.month) + 1


def _window(args: MonthlyBreakdownArgs, today: date) -> tuple[date, date]:
    if args.date_from is not None and args.date_to is not None:
        d_from = parse_date(args.date_from, field="date_from")
        d_to = parse_date(args.date_to, field="date_to")
        if d_from > d_to:
            raise BadArgs("date_from must be <= date_to")
        start, end = _first_of_month(d_from), _last_of_month(d_to)
        if _month_span(start, end) > _MAX_MONTHS:
            raise BadArgs("date range must span at most 24 months")
        return start, end
    start = _shift_months(today, -(args.months - 1))
    return start, _last_of_month(today)


async def execute(
    args: MonthlyBreakdownArgs, state: AppState, actor: Actor, visibility: Visibility
) -> dict[str, Any]:
    if not state.schema.payments_available:
        raise SourceUnavailable("payment data is not available yet")

    today = state.today()
    assignee_model = resolve_assignee(args.assignee_id, actor, visibility)
    start, end = _window(args, today)
    title = args.title_query

    sql = load_sql("monthly_breakdown.sql").format(
        base=render_occurrences_base(state), status_sql=STATUS_SQL[args.status]
    )
    rows = await state.db.ro_query(
        sql,
        today,                                  # $1
        start,                                  # $2
        end,                                    # $3
        args.currency,                          # $4
        title,                                  # $5 tsquery
        escape_like(title) if title else None,  # $6 ilike
        assignee_model,                         # $7
        visibility.own_filter,                  # $8
    )

    shaped: list[dict[str, Any]] = []
    for r in rows[:72]:
        total = Decimal(r["total"])
        shaped.append(
            {
                "month": r["month"],
                "currency": r["currency"],
                "total": money.to_amount_string(total),
                "paid": money.to_amount_string(Decimal(r["paid"])),
                "unpaid": money.to_amount_string(Decimal(r["unpaid"])),
                "count": int(r["cnt"]),
                "display": money.display(total, r["currency"]),
            }
        )

    coverage_until, coverage_incomplete = await compute_coverage(state, (start, end))
    return {
        "today": today.isoformat(),
        "visibility": visibility.scope,
        "from": start.isoformat(),
        "to": end.isoformat(),
        "rows": shaped,
        "coverage_until": coverage_until.isoformat() if coverage_until else None,
        "coverage_incomplete": coverage_incomplete,
    }
