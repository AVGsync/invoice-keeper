"""Idempotent creation of the read-only ``nlq_reader`` role and its grants.

    python -m nlq.dbinit            # role + grants (+ dev fixtures when local)
    python -m nlq.dbinit --drop-dev # drop the dev fixtures schema

Runs under the Postgres superuser after ``migrate``. Never touches
``backend/migrations`` and creates no objects that depend on B's tables (§6).
The password and CONNECT grant use ``format(%L)``/``format(%I)`` for safe
quoting; the dev-schema name is validated as an identifier.
"""

from __future__ import annotations

import argparse
import asyncio
import logging
import re
import sys
from pathlib import Path

import asyncpg

from .config import get_settings

log = logging.getLogger("nlq.dbinit")

_DB_DIR = Path(__file__).parent.parent.parent / "db"
_IDENT_RE = re.compile(r"^[a-z_][a-z0-9_]*$")


def _read(name: str) -> str:
    return (_DB_DIR / name).read_text(encoding="utf-8")


async def _exec_format(conn: asyncpg.Connection, template_sql: str, *args: object) -> None:
    """Build a statement server-side via SQL format() (safe quoting) and run it."""
    stmt = await conn.fetchval(template_sql, *args)
    if not isinstance(stmt, str):
        raise RuntimeError("format() did not return SQL text")
    await conn.execute(stmt)


async def _apply_role_and_grants(conn: asyncpg.Connection, db_password: str) -> None:
    # Static DDL: role, limits, revokes, column-level grants, B-table DO block.
    await conn.execute(_read("nlq_reader.sql"))

    # Password (needs runtime value → format %L).
    await _exec_format(
        conn,
        "SELECT format('ALTER ROLE nlq_reader WITH LOGIN PASSWORD %L', $1::text)",
        db_password,
    )

    # CONNECT on the current database (name via format %I).
    await _exec_format(
        conn,
        "SELECT format('GRANT CONNECT ON DATABASE %I TO nlq_reader', current_database())",
    )


async def _apply_dev_schema(conn: asyncpg.Connection, schema: str) -> None:
    if not _IDENT_RE.match(schema):
        raise ValueError(f"invalid NLQ_DEV_SCHEMA identifier: {schema!r}")
    await _exec_format(conn, "SELECT format('CREATE SCHEMA IF NOT EXISTS %I', $1::text)", schema)
    # Create fixtures inside the dev schema (unqualified names in the file).
    await conn.execute(f'SET search_path TO "{schema}", public')
    await conn.execute(_read("dev_b_tables.sql"))
    await conn.execute("RESET search_path")
    await _exec_format(
        conn, "SELECT format('GRANT USAGE ON SCHEMA %I TO nlq_reader', $1::text)", schema
    )
    await _exec_format(
        conn,
        "SELECT format('GRANT SELECT ON ALL TABLES IN SCHEMA %I TO nlq_reader', $1::text)",
        schema,
    )
    log.info("dev schema %s created with fixtures and SELECT grant", schema)


async def _drop_dev_schema(conn: asyncpg.Connection, schema: str) -> None:
    if not _IDENT_RE.match(schema):
        raise ValueError(f"invalid NLQ_DEV_SCHEMA identifier: {schema!r}")
    await _exec_format(
        conn, "SELECT format('DROP SCHEMA IF EXISTS %I CASCADE', $1::text)", schema
    )
    log.info("dev schema %s dropped", schema)


async def run(drop_dev: bool) -> None:
    settings = get_settings()
    conn = await asyncpg.connect(settings.resolved_admin_dsn())
    try:
        if drop_dev:
            schema = settings.nlq_dev_schema
            if not schema:
                log.warning("--drop-dev given but NLQ_DEV_SCHEMA is empty; nothing to do")
                return
            await _drop_dev_schema(conn, schema)
            return

        if not settings.nlq_db_password:
            raise ValueError("NLQ_DB_PASSWORD is required")
        await _apply_role_and_grants(conn, settings.nlq_db_password)
        log.info("nlq_reader role and grants applied")

        dev = settings.dev_schema
        if dev:
            await _apply_dev_schema(conn, dev)
    finally:
        await conn.close()


def main() -> None:
    logging.basicConfig(level="INFO", format="%(asctime)s %(levelname)s %(name)s %(message)s")
    parser = argparse.ArgumentParser(prog="nlq.dbinit")
    parser.add_argument("--drop-dev", action="store_true", help="drop the dev fixtures schema")
    args = parser.parse_args()
    try:
        asyncio.run(run(args.drop_dev))
    except Exception as exc:
        log.error("dbinit failed: %s", exc)
        sys.exit(1)


if __name__ == "__main__":
    main()
