"""find_recurring_payments: payment templates (subscriptions, rent) (§5.5)."""

from __future__ import annotations

from decimal import Decimal
from typing import Any

from ...common import money
from ...common.actor import Actor
from ...common.errors import SourceUnavailable
from ...common.sanitize import sanitize
from ...common.tool_specs import FindRecurringPaymentsArgs
from ..db import load_sql
from ..fragments import (
    DEFAULT_RECURRING_ORDER,
    RECURRING_ORDER_SQL,
    category_col,
    escape_like,
)
from ..policy import Visibility, resolve_assignee
from ..state import AppState
from ._common import category_filter


async def execute(
    args: FindRecurringPaymentsArgs, state: AppState, actor: Actor, visibility: Visibility
) -> dict[str, Any]:
    if not state.schema.payments_available:
        raise SourceUnavailable("payment data is not available yet")

    today = state.today()
    assignee_model = resolve_assignee(args.assignee_id, actor, visibility)
    min_amount = money.parse_amount(args.min_amount, field="min_amount") if args.min_amount is not None else None
    max_amount = money.parse_amount(args.max_amount, field="max_amount") if args.max_amount is not None else None
    title = args.title_query
    limit = min(args.limit or state.settings.nlq_max_rows, state.settings.nlq_max_rows)
    category, category_unavailable = category_filter(state, args.category)

    sql = load_sql("find_recurring_payments.sql").format(
        category_col=category_col(state.schema.category_available),
        order_sql=RECURRING_ORDER_SQL[args.sort or DEFAULT_RECURRING_ORDER],
    )
    rows = await state.db.ro_query(
        sql,
        today,                                  # $1
        args.include_ended,                     # $2
        args.currency,                          # $3
        min_amount,                             # $4
        max_amount,                             # $5
        assignee_model,                         # $6
        visibility.own_filter,                  # $7
        title,                                  # $8 tsquery
        escape_like(title) if title else None,  # $9 ilike
        limit,                                  # $10
        category,                               # $11
    )

    total = int(rows[0]["total_count"]) if rows else 0
    shaped: list[dict[str, Any]] = []
    for r in rows:
        amount = Decimal(r["amount"])
        currency = r["currency"]
        next_due = r["next_due"]
        shaped.append(
            {
                "payment_id": r["payment_id"],
                "title": sanitize(r["title"]),
                "amount": money.to_amount_string(amount),
                "currency": currency,
                "display": money.display(amount, currency),
                "assignee_id": r["assignee_id"],
                "assignee_name": sanitize(r["assignee_name"]) if r["assignee_name"] else None,
                "assignee_persona": sanitize(r["assignee_persona"]) if r["assignee_persona"] else None,
                "is_ended": r["is_ended"],
                "next_due": next_due.isoformat() if next_due else None,
            }
        )
    return {
        "today": today.isoformat(),
        "visibility": visibility.scope,
        "total": total,
        "truncated": total > len(shaped),
        "category_unavailable": category_unavailable,
        "rows": shaped,
    }
