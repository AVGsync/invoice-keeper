"""FastMCP server: the 4 curated read-only tools + /healthz (§5, §9.3).

Tool signatures mirror the pydantic models in ``common.tool_specs`` so FastMCP
publishes ``additionalProperties:false`` schemas; the bodies re-validate through
the same models (cross-field rules) and always return a JSON envelope string.
"""

from __future__ import annotations

import asyncio
import contextlib
import logging
from collections.abc import AsyncIterator
from contextlib import asynccontextmanager
from typing import Literal

from fastmcp import FastMCP
from starlette.requests import Request
from starlette.responses import JSONResponse

from ..common.tool_specs import Currency, PeriodArg, Status
from .selfcheck import periodic_recheck, refresh_schema_status, run_security_checks
from .state import get_state
from .tools import compare_periods as _cp
from .tools import find_occurrences as _fo
from .tools import find_people as _fp
from .tools import find_recurring_payments as _frp
from .tools import get_payment_details as _gpd
from .tools import list_personas as _lp
from .tools import monthly_breakdown as _mb
from .tools import sum_occurrences as _so
from .tools import workload as _wl
from .tools._common import run_tool, validate_args

log = logging.getLogger("nlq.mcp.server")


@asynccontextmanager
async def _lifespan(_server: FastMCP) -> AsyncIterator[None]:
    """Connect the pool, run fail-closed security checks, probe the schema,
    then keep the schema status fresh every 60 s (§5.9)."""
    state = get_state()
    await state.db.connect()
    await run_security_checks(state.db)  # raises -> startup aborts (fail-closed)
    await refresh_schema_status(state)
    log.info("mcp ready: payments_schema=%s", state.schema.value)
    task = asyncio.create_task(periodic_recheck(state))
    try:
        yield
    finally:
        task.cancel()
        with contextlib.suppress(asyncio.CancelledError):
            await task
        await state.db.close()


mcp = FastMCP(name="invoice-keeper-nlq", lifespan=_lifespan)


@mcp.tool
async def find_occurrences(
    status: Status,
    period: PeriodArg | None = None,
    date_from: str | None = None,
    date_to: str | None = None,
    currency: Currency | None = None,
    min_amount: float | None = None,
    max_amount: float | None = None,
    title_query: str | None = None,
    category: str | None = None,
    assignee_id: str | None = None,
    sort: Literal["due_date", "amount_desc"] | None = None,
    limit: int | None = None,
) -> str:
    """Найти периоды оплаты (начисления): просрочено, скоро, оплачено."""
    args = validate_args(
        "find_occurrences",
        {
            "status": status, "period": period, "date_from": date_from, "date_to": date_to,
            "currency": currency, "min_amount": min_amount, "max_amount": max_amount,
            "title_query": title_query, "category": category, "assignee_id": assignee_id,
            "sort": sort, "limit": limit,
        },
    )
    return await run_tool("find_occurrences", lambda s, a, v: _fo.execute(args, s, a, v))


@mcp.tool
async def sum_occurrences(
    status: Status,
    period: PeriodArg | None = None,
    date_from: str | None = None,
    date_to: str | None = None,
    currency: Currency | None = None,
    title_query: str | None = None,
    category: str | None = None,
    assignee_id: str | None = None,
    group_by: Literal["currency", "currency_assignee"] = "currency",
    basis: Literal["period_sum", "run_rate"] = "period_sum",
) -> str:
    """Суммы периодов отдельно по каждой валюте. Курсы не применяются."""
    args = validate_args(
        "sum_occurrences",
        {
            "status": status, "period": period, "date_from": date_from, "date_to": date_to,
            "currency": currency, "title_query": title_query, "category": category,
            "assignee_id": assignee_id, "group_by": group_by, "basis": basis,
        },
    )
    return await run_tool("sum_occurrences", lambda s, a, v: _so.execute(args, s, a, v))


@mcp.tool
async def find_recurring_payments(
    title_query: str | None = None,
    category: str | None = None,
    currency: Currency | None = None,
    min_amount: float | None = None,
    max_amount: float | None = None,
    assignee_id: str | None = None,
    include_ended: bool = False,
    sort: Literal["title", "amount_desc", "next_due"] | None = None,
    limit: int | None = None,
) -> str:
    """Найти регулярные платежи (подписки, аренда, сервисы)."""
    args = validate_args(
        "find_recurring_payments",
        {
            "title_query": title_query, "category": category, "currency": currency,
            "min_amount": min_amount, "max_amount": max_amount, "assignee_id": assignee_id,
            "include_ended": include_ended, "sort": sort, "limit": limit,
        },
    )
    return await run_tool("find_recurring_payments", lambda s, a, v: _frp.execute(args, s, a, v))


@mcp.tool
async def find_people(query: str | None = None, limit: int | None = None) -> str:
    """Найти сотрудников по имени или должности и получить их id."""
    args = validate_args("find_people", {"query": query, "limit": limit})
    return await run_tool("find_people", lambda s, a, v: _fp.execute(args, s, a, v))


@mcp.tool
async def get_payment_details(
    title_query: str,
    assignee_id: str | None = None,
    include_ended: bool = False,
) -> str:
    """Детали регулярного платежа по названию: срок, последняя оплата, счётчики."""
    args = validate_args(
        "get_payment_details",
        {
            "title_query": title_query, "assignee_id": assignee_id,
            "include_ended": include_ended,
        },
    )
    return await run_tool("get_payment_details", lambda s, a, v: _gpd.execute(args, s, a, v))


@mcp.tool
async def monthly_breakdown(
    status: Status,
    months: int = 6,
    date_from: str | None = None,
    date_to: str | None = None,
    currency: Currency | None = None,
    assignee_id: str | None = None,
    title_query: str | None = None,
) -> str:
    """Суммы по месяцам и валютам: динамика расходов. Валюты не смешиваются."""
    args = validate_args(
        "monthly_breakdown",
        {
            "status": status, "months": months, "date_from": date_from, "date_to": date_to,
            "currency": currency, "assignee_id": assignee_id, "title_query": title_query,
        },
    )
    return await run_tool("monthly_breakdown", lambda s, a, v: _mb.execute(args, s, a, v))


@mcp.tool
async def compare_periods(
    status: Status,
    period_a: PeriodArg,
    period_b: PeriodArg,
    a_from: str | None = None,
    a_to: str | None = None,
    b_from: str | None = None,
    b_to: str | None = None,
    currency: Currency | None = None,
    assignee_id: str | None = None,
    title_query: str | None = None,
) -> str:
    """Сравнить два периода по каждой валюте (A — раньше, B — позже)."""
    args = validate_args(
        "compare_periods",
        {
            "status": status, "period_a": period_a, "period_b": period_b,
            "a_from": a_from, "a_to": a_to, "b_from": b_from, "b_to": b_to,
            "currency": currency, "assignee_id": assignee_id, "title_query": title_query,
        },
    )
    return await run_tool("compare_periods", lambda s, a, v: _cp.execute(args, s, a, v))


@mcp.tool
async def workload(
    group_by: Literal["assignee", "persona"],
    status: Status = "unpaid",
    period: PeriodArg | None = None,
    date_from: str | None = None,
    date_to: str | None = None,
    currency: Currency | None = None,
) -> str:
    """Нагрузка по ответственным или должностям: число платежей и суммы."""
    args = validate_args(
        "workload",
        {
            "group_by": group_by, "status": status, "period": period,
            "date_from": date_from, "date_to": date_to, "currency": currency,
        },
    )
    return await run_tool("workload", lambda s, a, v: _wl.execute(args, s, a, v))


@mcp.tool
async def list_personas(query: str | None = None, limit: int | None = None) -> str:
    """Список должностей: сколько сотрудников и активных платежей на каждой."""
    args = validate_args("list_personas", {"query": query, "limit": limit})
    return await run_tool("list_personas", lambda s, a, v: _lp.execute(args, s, a, v))


@mcp.custom_route("/healthz", methods=["GET"])
async def healthz(_request: Request) -> JSONResponse:
    state = get_state()
    schema = state.schema
    db_ok = await _db_ping()
    healthy = db_ok and schema.payments_available
    reason = None
    if not db_ok:
        reason = "db_down"
    elif not schema.payments_available:
        reason = schema.reason
    return JSONResponse(
        {
            "status": "ok" if healthy else "degraded",
            "db": "ok" if db_ok else "down",
            "grants": "ok",
            "payments_schema": schema.value,
            "reason": reason,
        }
    )


async def _db_ping() -> bool:
    try:
        return await get_state().db.ro_fetchval("SELECT 1") == 1
    except Exception:
        return False
