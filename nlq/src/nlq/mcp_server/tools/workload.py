"""workload: нагрузка по ответственным или должностям (§D.7).

Для каждого человека (или должности) — число платежей и суммы по каждой валюте
отдельно, плюс просроченные. Группа выбирается кодом (не из текста). Блок —
``table`` (``kind:"workload"``).
"""

from __future__ import annotations

from decimal import Decimal
from typing import Any

from ...common import money
from ...common.actor import Actor
from ...common.dates import resolve_period
from ...common.errors import SourceUnavailable
from ...common.sanitize import sanitize
from ...common.tool_specs import WorkloadArgs
from ..db import load_sql
from ..fragments import STATUS_SQL
from ..policy import Visibility
from ..state import AppState
from ._common import render_occurrences_base

# Код выбирает выражение группировки — пользовательский текст сюда не попадает.
_GROUP_NAME = {
    "assignee": "COALESCE(f.u_name, '—')",
    "persona": "COALESCE(f.p_name, 'Без должности')",
}


async def execute(
    args: WorkloadArgs, state: AppState, actor: Actor, visibility: Visibility
) -> dict[str, Any]:
    if not state.schema.payments_available:
        raise SourceUnavailable("payment data is not available yet")

    today = state.today()
    bounds = resolve_period(args.period, today, args.date_from, args.date_to)

    sql = load_sql("workload.sql").format(
        base=render_occurrences_base(state),
        status_sql=STATUS_SQL[args.status],
        group_name=_GROUP_NAME[args.group_by],
    )
    rows = await state.db.ro_query(
        sql,
        today,                          # $1
        bounds[0] if bounds else None,  # $2
        bounds[1] if bounds else None,  # $3
        args.currency,                  # $4
        visibility.own_filter,          # $5
    )

    groups_truncated = bool(rows) and int(rows[0]["total_groups"]) > len(rows)
    shaped: list[dict[str, Any]] = []
    for r in rows:
        total = Decimal(r["total"])
        shaped.append(
            {
                "name": sanitize(r["name"]) if r["name"] else None,
                "currency": r["currency"],
                "count": int(r["cnt"]),
                "total": money.to_amount_string(total),
                "overdue_count": int(r["overdue_count"]),
                "display": money.display(total, r["currency"]),
            }
        )

    return {
        "today": today.isoformat(),
        "visibility": visibility.scope,
        "group_by": args.group_by,
        "rows": shaped,
        "groups_truncated": groups_truncated,
    }
