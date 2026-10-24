"""sum_occurrences: totals strictly per currency, never mixed."""

from __future__ import annotations

from decimal import Decimal
from typing import Any

from ...common import money
from ...common.actor import Actor
from ...common.dates import resolve_period
from ...common.errors import SourceUnavailable
from ...common.tool_specs import SumOccurrencesArgs
from ..db import load_sql
from ..fragments import STATUS_SQL, escape_like
from ..policy import Visibility, resolve_assignee
from ..state import AppState
from ._common import (
    category_filter,
    compute_coverage,
    period_block,
    render_occurrences_base,
)

_GROUPS_LIMIT = 60


async def execute(
    args: SumOccurrencesArgs, state: AppState, actor: Actor, visibility: Visibility
) -> dict[str, Any]:
    if not state.schema.payments_available:
        raise SourceUnavailable("payment data is not available yet")

    today = state.today()
    assignee_model = resolve_assignee(args.assignee_id, actor, visibility)
    category, category_unavailable = category_filter(state, args.category)

    if args.basis == "run_rate":
        return await _run_rate(state, args, assignee_model, visibility, category_unavailable)

    bounds = resolve_period(args.period, today, args.date_from, args.date_to)
    title = args.title_query
    base = render_occurrences_base(state)
    params = (
        today,                                  # $1
        bounds[0] if bounds else None,          # $2
        bounds[1] if bounds else None,          # $3
        args.currency,                          # $4
        title,                                  # $5 tsquery
        escape_like(title) if title else None,  # $6 ilike
        assignee_model,                         # $7
        visibility.own_filter,                  # $8
        category,                               # $9
    )

    by_sql = load_sql("sum_occurrences.sql").format(
        base=base, status_sql=STATUS_SQL[args.status]
    )
    by_rows = await state.db.ro_query(by_sql, *params)
    by_currency_out = {
        r["currency"]: {
            "total": money.to_amount_string(Decimal(r["total"])),
            "paid": money.to_amount_string(Decimal(r["paid_total"])),
            "unpaid": money.to_amount_string(Decimal(r["unpaid_total"])),
            "count": int(r["cnt"]),
            "display": money.display(Decimal(r["total"]), r["currency"]),
        }
        for r in sorted(by_rows, key=lambda r: r["currency"])
    }

    groups: list[dict[str, Any]] = []
    groups_truncated = False
    if args.group_by == "currency_assignee":
        groups_sql = load_sql("sum_occurrences_groups.sql").format(
            base=base, status_sql=STATUS_SQL[args.status]
        )
        g_rows = await state.db.ro_query(groups_sql, *params)
        if g_rows:
            groups_truncated = int(g_rows[0]["total_groups"]) > len(g_rows)
        for r in g_rows:
            total = Decimal(r["total"])
            groups.append(
                {
                    "currency": r["currency"],
                    "assignee_id": r["assignee_id"],
                    "assignee_name": r["assignee_name"],
                    "total": money.to_amount_string(total),
                    "paid": money.to_amount_string(Decimal(r["paid_total"])),
                    "unpaid": money.to_amount_string(Decimal(r["unpaid_total"])),
                    "count": int(r["cnt"]),
                    "display": money.display(total, r["currency"]),
                }
            )

    coverage_until, coverage_incomplete = await compute_coverage(state, bounds)
    return {
        "basis": "period_sum",
        "by_currency": by_currency_out,
        "groups": groups,
        "groups_truncated": groups_truncated,
        "period": period_block(bounds),
        "coverage_until": coverage_until.isoformat() if coverage_until else None,
        "coverage_incomplete": coverage_incomplete,
        "category_unavailable": category_unavailable,
    }


async def _run_rate(
    state: AppState,
    args: SumOccurrencesArgs,
    assignee_model: Any,
    visibility: Visibility,
    category_unavailable: bool,
) -> dict[str, Any]:
    rows = await state.db.ro_query(
        load_sql("run_rate.sql"),
        args.currency,          # $1
        assignee_model,         # $2
        visibility.own_filter,  # $3
    )
    by_currency_out = {
        r["currency"]: {
            "total": money.to_amount_string(Decimal(r["total"])),
            "count": int(r["cnt"]),
            "display": money.display(Decimal(r["total"]), r["currency"]),
        }
        for r in sorted(rows, key=lambda r: r["currency"])
    }
    return {
        "basis": "run_rate",
        "by_currency": by_currency_out,
        "groups": [],
        "groups_truncated": False,
        "period": None,
        "coverage_until": None,
        "coverage_incomplete": False,
        "category_unavailable": category_unavailable,
    }
