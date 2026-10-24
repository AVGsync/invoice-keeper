"""list_personas: должности и сколько на каждой сотрудников/активных платежей
(§D.7). Блок — ``table`` (``kind:"personas"``).

Работает и без таблиц B: ``active_payments`` становится 0 через код-фрагмент.
Под ``own``-видимостью счётчики активных платежей по чужим должностям не
раскрываются (``null``), как в find_people.
"""

from __future__ import annotations

from typing import Any

from ...common.actor import Actor
from ...common.sanitize import sanitize
from ...common.tool_specs import ListPersonasArgs
from ..db import load_sql
from ..fragments import escape_like
from ..policy import Visibility
from ..state import AppState

# Активные регулярные платежи сотрудников этой должности (decision #2/#15).
# Коррелированный подсчёт по p.id; код выбирает его либо литерал 0.
ACTIVE_PAYMENTS_SUBQUERY = (
    "(SELECT count(*) FROM user_recurrent_payments ur "
    "JOIN recurrent_payments rp ON rp.id = ur.recurrent_payment_id "
    "JOIN users uu ON uu.id = ur.user_id "
    "WHERE uu.persona_id = p.id AND rp.stopped_at IS NULL)"
)


async def execute(
    args: ListPersonasArgs, state: AppState, actor: Actor, visibility: Visibility
) -> dict[str, Any]:
    query = args.query
    limit = min(args.limit or state.settings.nlq_max_rows, state.settings.nlq_max_rows)

    active_expr = ACTIVE_PAYMENTS_SUBQUERY if state.schema.payments_available else "0"
    sql = load_sql("list_personas.sql").format(active_payments=active_expr)
    rows = await state.db.ro_query(
        sql,
        query,                                  # $1 tsquery
        escape_like(query) if query else None,  # $2 ilike
        limit,                                  # $3
    )

    own = visibility.scope == "own"
    shaped: list[dict[str, Any]] = []
    for r in rows:
        shaped.append(
            {
                "persona": sanitize(r["persona"]) if r["persona"] else None,
                "users_count": int(r["users_count"]),
                # Под own-видимостью чужие счётчики не раскрываем (§5.6).
                "active_payments": None if own else int(r["active_payments"]),
            }
        )
    return {
        "visibility": visibility.scope,
        "total": len(shaped),
        "truncated": False,
        "rows": shaped,
    }
