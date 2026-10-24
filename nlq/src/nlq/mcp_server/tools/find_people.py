"""find_people: users and personas."""

from __future__ import annotations

from typing import Any

from ...common.actor import Actor
from ...common.sanitize import sanitize
from ...common.tool_specs import FindPeopleArgs
from ..db import load_sql
from ..fragments import escape_like
from ..policy import Visibility
from ..state import AppState

ACTIVE_PAYMENTS_SUBQUERY = (
    "(SELECT count(*) FROM user_recurrent_payments ur "
    "JOIN recurrent_payments rp ON rp.id = ur.recurrent_payment_id "
    "WHERE ur.user_id = u.id AND rp.stopped_at IS NULL)"
)


async def execute(
    args: FindPeopleArgs, state: AppState, actor: Actor, visibility: Visibility
) -> dict[str, Any]:
    query = args.query
    limit = min(args.limit or state.settings.nlq_max_rows, state.settings.nlq_max_rows)

    active_expr = (
        ACTIVE_PAYMENTS_SUBQUERY if state.schema.payments_available else "0"
    )
    sql = load_sql("find_people.sql").format(active_payments=active_expr)
    rows = await state.db.ro_query(
        sql,
        query,                                  # $1 tsquery
        escape_like(query) if query else None,  # $2 ilike
        limit,                                  # $3
    )

    shaped: list[dict[str, Any]] = []
    for r in rows:
        user_id = r["user_id"]
        active = int(r["active_payments"])
        # Under own scope, do not report counts for other people.
        if visibility.scope == "own" and user_id != str(actor.sub):
            active = None  # type: ignore[assignment]
        shaped.append(
            {
                "user_id": user_id,
                "full_name": sanitize(r["full_name"]),
                "role": r["role"],
                "persona": sanitize(r["persona"]) if r["persona"] else None,
                "active_payments": active,
            }
        )
    return {
        "visibility": visibility.scope,
        "total": len(shaped),
        "truncated": False,
        "rows": shaped,
    }
