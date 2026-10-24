"""get_payment_details: детали регулярного платежа по названию."""

from __future__ import annotations

from decimal import Decimal
from typing import Any

from ...common import money
from ...common.actor import Actor
from ...common.errors import SourceUnavailable
from ...common.sanitize import sanitize
from ...common.tool_specs import GetPaymentDetailsArgs
from ..db import load_sql
from ..fragments import escape_like
from ..policy import Visibility, resolve_assignee
from ..state import AppState


def _iso(value: Any) -> str | None:
    return value.isoformat() if value else None


async def execute(
    args: GetPaymentDetailsArgs, state: AppState, actor: Actor, visibility: Visibility
) -> dict[str, Any]:
    if not state.schema.payments_available:
        raise SourceUnavailable("payment data is not available yet")

    today = state.today()
    assignee_model = resolve_assignee(args.assignee_id, actor, visibility)
    title = args.title_query

    rows = await state.db.ro_query(
        load_sql("get_payment_details.sql"),
        today,                      # $1
        args.include_ended,         # $2
        assignee_model,             # $3
        visibility.own_filter,      # $4
        title,                      # $5 tsquery
        escape_like(title),         # $6 ilike (escaped)
    )

    total = int(rows[0]["total_count"]) if rows else 0
    shaped: list[dict[str, Any]] = []
    for r in rows:
        amount = Decimal(r["amount"])
        currency = r["currency"]
        shaped.append(
            {
                "payment_id": r["payment_id"],
                "title": sanitize(r["title"]),
                "amount": money.to_amount_string(amount),
                "currency": currency,
                "display": money.display(amount, currency),
                "frequency": r["frequency"],
                "interval": int(r["interval"]),
                "start_date": _iso(r["start_date"]),
                "is_ended": r["is_ended"],
                "assignee_id": r["assignee_id"],
                "assignee_name": sanitize(r["assignee_name"]) if r["assignee_name"] else None,
                "assignee_persona": sanitize(r["assignee_persona"]) if r["assignee_persona"] else None,
                "next_due": _iso(r["next_due"]),
                "last_paid_date": _iso(r["last_paid_date"]),
                "overdue_count": int(r["overdue_count"]),
                "paid_count_12m": int(r["paid_count_12m"]),
            }
        )
    return {
        "today": today.isoformat(),
        "visibility": visibility.scope,
        "total": total,
        "truncated": total > len(shaped),
        "rows": shaped,
    }
