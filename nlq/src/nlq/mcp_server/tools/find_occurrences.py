"""find_occurrences: payment periods (occurrences) with filters."""

from __future__ import annotations

from typing import Any

from ...common import money
from ...common.actor import Actor
from ...common.dates import resolve_period
from ...common.errors import SourceUnavailable
from ...common.tool_specs import FindOccurrencesArgs
from ..db import load_sql
from ..fragments import (
    DEFAULT_OCCURRENCE_ORDER,
    OCCURRENCE_ORDER_SQL,
    STATUS_SQL,
    escape_like,
)
from ..policy import Visibility, resolve_assignee
from ..state import AppState
from ._common import (
    category_filter,
    compute_coverage,
    occurrence_row,
    period_block,
    render_occurrences_base,
)


async def execute(
    args: FindOccurrencesArgs, state: AppState, actor: Actor, visibility: Visibility
) -> dict[str, Any]:
    if not state.schema.payments_available:
        raise SourceUnavailable("payment data is not available yet")

    today = state.today()
    bounds = resolve_period(args.period, today, args.date_from, args.date_to)
    assignee_model = resolve_assignee(args.assignee_id, actor, visibility)

    min_amount = money.parse_amount(args.min_amount, field="min_amount") if args.min_amount is not None else None
    max_amount = money.parse_amount(args.max_amount, field="max_amount") if args.max_amount is not None else None
    title = args.title_query
    limit = min(args.limit or state.settings.nlq_max_rows, state.settings.nlq_max_rows)
    category, category_unavailable = category_filter(state, args.category)

    sql = load_sql("find_occurrences.sql").format(
        base=render_occurrences_base(state),
        status_sql=STATUS_SQL[args.status],
        order_sql=OCCURRENCE_ORDER_SQL[args.sort or DEFAULT_OCCURRENCE_ORDER],
    )
    rows = await state.db.ro_query(
        sql,
        today,                                  # $1
        bounds[0] if bounds else None,          # $2
        bounds[1] if bounds else None,          # $3
        args.currency,                          # $4
        min_amount,                             # $5
        max_amount,                             # $6
        assignee_model,                         # $7
        visibility.own_filter,                  # $8
        title,  # $9 tsquery
        escape_like(title) if title else None,  # $10 ilike
        limit,                                  # $11
        category,                               # $12
    )

    total = int(rows[0]["total_count"]) if rows else 0
    shaped = [occurrence_row(r, today) for r in rows]
    coverage_until, coverage_incomplete = await compute_coverage(state, bounds)
    return {
        "today": today.isoformat(),
        "visibility": visibility.scope,
        "period": period_block(bounds),
        "total": total,
        "truncated": total > len(shaped),
        "coverage_until": coverage_until.isoformat() if coverage_until else None,
        "coverage_incomplete": coverage_incomplete,
        "category_unavailable": category_unavailable,
        "rows": shaped,
    }
