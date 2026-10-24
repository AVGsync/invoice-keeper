"""Fail-closed self-checks run before serving, and a periodic schema recheck (§5.9).

Security checks 1-5 verify the role really is a least-privilege reader; any
failure means the process must not serve (the caller exits non-zero). Check 6
(payment-schema probe) is soft: the process starts, but ``/healthz`` reports
``degraded`` and payment tools answer ``source_unavailable`` until B's tables
appear. It re-runs every 60 s so a merge needs no restart.
"""

from __future__ import annotations

import asyncio
import logging
from typing import Any

import asyncpg

from .db import Database, load_sql
from .fragments import (
    OCCURRENCE_ORDER_SQL,
    RECURRING_ORDER_SQL,
    STATUS_SQL,
    category_col,
)
from .state import (
    SCHEMA_BROKEN,
    SCHEMA_DRIFT,
    SCHEMA_MISSING,
    SCHEMA_OK,
    AppState,
    SchemaStatus,
)

_CATEGORY_PROBE = """
SELECT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = 'recurring_payments'
      AND column_name = 'category'
      AND table_schema = ANY (current_schemas(false))
)
"""


def _rendered_templates(category_available: bool) -> list[str]:
    """Every final SQL text a tool can run, so prepare() validates them all."""
    base = load_sql("occurrences_base.sql").format(
        category_col=category_col(category_available)
    )
    out: list[str] = []
    for status in STATUS_SQL.values():
        for order in OCCURRENCE_ORDER_SQL.values():
            out.append(
                load_sql("find_occurrences.sql").format(
                    base=base, status_sql=status, order_sql=order
                )
            )
        out.append(load_sql("sum_occurrences.sql").format(base=base, status_sql=status))
        out.append(
            load_sql("sum_occurrences_groups.sql").format(base=base, status_sql=status)
        )
    for order in RECURRING_ORDER_SQL.values():
        out.append(
            load_sql("find_recurring_payments.sql").format(
                category_col=category_col(category_available), order_sql=order
            )
        )
    for active in (
        "(SELECT count(*) FROM recurring_payments rp "
        "WHERE rp.assignee_id = u.id AND rp.ended_at IS NULL)",
        "0",
    ):
        out.append(load_sql("find_people.sql").format(active_payments=active))
    out.append(load_sql("coverage.sql"))
    out.append(load_sql("run_rate.sql"))
    return out

log = logging.getLogger("nlq.mcp.selfcheck")

RECHECK_INTERVAL_SECONDS = 60


class SelfCheckError(Exception):
    """A hard security self-check failed; the process must not serve."""


async def _expect_denied(conn: Any, sql: str, label: str) -> None:
    """Pass if the query is denied (no grant) or the table is absent."""
    try:
        await conn.execute(sql)
    except asyncpg.InsufficientPrivilegeError:
        return
    except asyncpg.UndefinedTableError:
        # Cannot leak a table that does not exist yet; acceptable.
        return
    raise SelfCheckError(f"role can read forbidden data: {label}")


async def run_security_checks(db: Database) -> None:
    async with db.pool.acquire() as conn:
        row = await conn.fetchrow(
            "SELECT rolsuper, rolbypassrls FROM pg_roles WHERE rolname = current_user"
        )
        if row is None or row["rolsuper"] or row["rolbypassrls"]:
            raise SelfCheckError("role is superuser or bypasses RLS")

        await _expect_denied(conn, "SELECT password_hash FROM users LIMIT 0", "users.password_hash")
        await _expect_denied(conn, "SELECT login FROM users LIMIT 0", "users.login")
        await _expect_denied(conn, "SELECT 1 FROM refresh_tokens LIMIT 0", "refresh_tokens")
        await _expect_denied(conn, "SELECT 1 FROM telegram_links LIMIT 0", "telegram_links")

        can_write = await conn.fetchval(
            "SELECT has_table_privilege(current_user, 'public.users', 'INSERT,UPDATE,DELETE')"
        )
        if can_write:
            raise SelfCheckError("role can write to public.users")

    async with db.pool.acquire() as conn:
        async with conn.transaction(readonly=True):
            ro = await conn.fetchval("SHOW transaction_read_only")
        if ro != "on":
            raise SelfCheckError("transaction is not read-only")


async def probe_schema(db: Database) -> SchemaStatus:
    """Prepare *every* rendered tool template and classify the result.

    A syntax error or indeterminate-parameter error in any status/sort/group_by
    combination means an adapter is broken: /healthz goes degraded and payment
    tools fail closed, instead of reporting healthy while every call errors.
    """
    async with db.pool.acquire() as conn:
        try:
            category_available = bool(await conn.fetchval(_CATEGORY_PROBE))
        except asyncpg.PostgresError:
            category_available = False
        for sql in _rendered_templates(category_available):
            try:
                await conn.prepare(sql)
            except asyncpg.UndefinedTableError:
                return SchemaStatus(SCHEMA_MISSING, "payments_schema_missing")
            except asyncpg.UndefinedColumnError:
                return SchemaStatus(SCHEMA_DRIFT, "payments_schema_drift")
            except asyncpg.PostgresError as exc:
                log.error("adapter template failed to prepare: %s", exc.__class__.__name__)
                return SchemaStatus(SCHEMA_BROKEN, "adapter_broken")
    return SchemaStatus(SCHEMA_OK, None, category_available=category_available)


async def refresh_schema_status(state: AppState) -> None:
    state.schema = await probe_schema(state.db)


async def periodic_recheck(state: AppState) -> None:
    while True:
        await asyncio.sleep(RECHECK_INTERVAL_SECONDS)
        try:
            await refresh_schema_status(state)
        except Exception as exc:
            log.warning("periodic schema recheck failed: %s", exc.__class__.__name__)
