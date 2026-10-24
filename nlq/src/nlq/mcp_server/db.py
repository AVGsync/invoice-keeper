"""asyncpg access for the MCP server.

A small pool (1-4) under the ``nlq_reader`` role. Every query runs inside a
READ ONLY, repeatable-read transaction (defence in depth on top of the role's
grants). Postgres error text is mapped to a stable code and never surfaced to
the model or the user — only logged (§5.1 p.8).
"""

from __future__ import annotations

import logging
from functools import cache
from pathlib import Path
from typing import Any

import asyncpg

from ..common.errors import DbUnavailable, Forbidden, SourceUnavailable, Timeout

log = logging.getLogger("nlq.mcp.db")

_SQL_DIR = Path(__file__).parent / "sql"


@cache
def load_sql(name: str) -> str:
    """Load and cache a .sql file from the sql/ directory."""
    return (_SQL_DIR / name).read_text(encoding="utf-8")


class Database:
    def __init__(self, dsn: str, dev_schema: str | None = None) -> None:
        self._dsn = dsn
        self._dev_schema = dev_schema
        self._pool: asyncpg.Pool | None = None

    async def connect(self) -> None:
        server_settings: dict[str, str] = {}
        if self._dev_schema:
            # Unqualified names resolve to the dev fixtures first, then public.
            server_settings["search_path"] = f"{self._dev_schema}, public"
        self._pool = await asyncpg.create_pool(
            self._dsn,
            min_size=1,
            max_size=4,
            command_timeout=5,
            server_settings=server_settings or None,
        )

    async def close(self) -> None:
        if self._pool is not None:
            await self._pool.close()
            self._pool = None

    @property
    def pool(self) -> asyncpg.Pool:
        if self._pool is None:
            raise DbUnavailable("database pool is not initialised")
        return self._pool

    async def ro_query(self, sql: str, *params: Any) -> list[asyncpg.Record]:
        """Run a SELECT in a READ ONLY, repeatable-read transaction."""
        try:
            async with (
                self.pool.acquire() as conn,
                conn.transaction(readonly=True, isolation="repeatable_read"),
            ):
                return await conn.fetch(sql, *params)
        except Exception as exc:
            raise _map_error(exc) from exc

    async def ro_fetchval(self, sql: str, *params: Any) -> Any:
        try:
            async with (
                self.pool.acquire() as conn,
                conn.transaction(readonly=True, isolation="repeatable_read"),
            ):
                return await conn.fetchval(sql, *params)
        except Exception as exc:
            raise _map_error(exc) from exc


def _map_error(exc: Exception) -> Exception:
    if isinstance(exc, (DbUnavailable, Forbidden, SourceUnavailable, Timeout)):
        return exc
    if isinstance(exc, asyncpg.QueryCanceledError):
        return Timeout("query exceeded the time limit")
    if isinstance(exc, asyncpg.InsufficientPrivilegeError):
        # A missing grant is a security-relevant surprise: alert, do not leak.
        log.error("nlq_reader lacks a grant it should not need: %s", exc.__class__.__name__)
        return Forbidden("access to this data is not allowed")
    if isinstance(exc, (asyncpg.UndefinedTableError, asyncpg.UndefinedColumnError)):
        log.warning("payments schema missing/drifted: %s", exc.__class__.__name__)
        return SourceUnavailable("payment data is not available yet")
    if isinstance(exc, (asyncpg.PostgresConnectionError, ConnectionError, OSError, TimeoutError)):
        log.error("database connection problem: %s", exc.__class__.__name__)
        return DbUnavailable("database is unavailable")
    if isinstance(exc, asyncpg.PostgresError):
        log.error("unexpected postgres error: %s", exc.__class__.__name__)
        return DbUnavailable("database error")
    log.error("unexpected db-layer error: %s", exc.__class__.__name__)
    return DbUnavailable("database error")
