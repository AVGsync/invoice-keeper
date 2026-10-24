"""Grant and read-only enforcement against a real Postgres (§12.1 security row).

Skipped unless TEST_DATABASE_URL points at a SUPERUSER connection (CREATE ROLE
is required). Builds the real db/nlq_reader.sql role, then connects AS that role
and proves the secret columns and write paths are denied.
"""

from __future__ import annotations

import os
from pathlib import Path
from typing import cast

import asyncpg
import pytest

from nlq.mcp_server.db import Database
from nlq.mcp_server.selfcheck import run_security_checks

pytestmark = pytest.mark.db

READER_PASSWORD = "test_reader_pw"
DB_DIR = Path(__file__).parent.parent.parent / "db"

_SECRET_TABLES_DDL = """
CREATE TABLE IF NOT EXISTS personas (id UUID PRIMARY KEY DEFAULT gen_random_uuid(), name TEXT NOT NULL, description TEXT);
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), full_name TEXT NOT NULL,
    login TEXT NOT NULL, password_hash TEXT NOT NULL, role TEXT NOT NULL,
    persona_id UUID REFERENCES personas(id) ON DELETE SET NULL
);
CREATE TABLE IF NOT EXISTS refresh_tokens (id UUID PRIMARY KEY DEFAULT gen_random_uuid(), token_hash TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS telegram_links (id UUID PRIMARY KEY DEFAULT gen_random_uuid(), tg_user_id BIGINT NOT NULL);
"""


async def _build_reader(admin: asyncpg.Connection) -> None:
    await admin.execute(_SECRET_TABLES_DDL)
    await admin.execute((DB_DIR / "nlq_reader.sql").read_text())
    pw_sql = cast(str, await admin.fetchval(
        "SELECT format('ALTER ROLE nlq_reader WITH LOGIN PASSWORD %L', $1::text)", READER_PASSWORD
    ))
    await admin.execute(pw_sql)
    grant_sql = cast(str, await admin.fetchval(
        "SELECT format('GRANT CONNECT ON DATABASE %I TO nlq_reader', current_database())"
    ))
    await admin.execute(grant_sql)


async def _reader_conn(dsn: str) -> asyncpg.Connection:
    return await asyncpg.connect(dsn, user="nlq_reader", password=READER_PASSWORD)


@pytest.fixture
def dsn() -> str:
    value = os.environ.get("TEST_DATABASE_URL")
    if not value:
        pytest.skip("TEST_DATABASE_URL not set")
    # Refuse destructive grant tests on a non-test database (see conftest).
    if not os.environ.get("NLQ_ALLOW_DESTRUCTIVE_TESTS"):
        name = value.rsplit("/", 1)[-1].split("?", 1)[0]
        if not name.endswith("_test"):
            pytest.skip(
                f"refusing destructive DB tests on {name!r}: use a '*_test' database "
                "or set NLQ_ALLOW_DESTRUCTIVE_TESTS=1"
            )
    return value


async def test_secret_columns_and_writes_denied(dsn: str) -> None:
    admin = await asyncpg.connect(dsn)
    try:
        await _build_reader(admin)
    finally:
        await admin.close()

    reader = await _reader_conn(dsn)
    try:
        # allow-listed columns work
        await reader.execute("SELECT id, full_name, role, persona_id FROM users LIMIT 0")

        with pytest.raises(asyncpg.InsufficientPrivilegeError):
            await reader.execute("SELECT password_hash FROM users LIMIT 1")
        with pytest.raises(asyncpg.InsufficientPrivilegeError):
            await reader.execute("SELECT login FROM users LIMIT 1")
        with pytest.raises(asyncpg.InsufficientPrivilegeError):
            await reader.execute("SELECT 1 FROM refresh_tokens LIMIT 1")
        with pytest.raises(asyncpg.InsufficientPrivilegeError):
            await reader.execute("SELECT 1 FROM telegram_links LIMIT 1")

        # Writes denied by the GRANTs even after read-only is dropped: run the
        # SET as its own statement so the INSERT is not refused merely because it
        # shares an already-read-only implicit transaction (this exercises the
        # grant-only defence section 6.1/section 12.1 rely on).
        await reader.execute("SET default_transaction_read_only = off")
        with pytest.raises(asyncpg.InsufficientPrivilegeError):
            await reader.execute(
                "INSERT INTO users (full_name, login, password_hash, role) "
                "VALUES ('x','y','z','user')"
            )
        with pytest.raises(asyncpg.InsufficientPrivilegeError):
            await reader.execute("UPDATE users SET role = 'admin'")
        with pytest.raises(asyncpg.InsufficientPrivilegeError):
            await reader.execute("DELETE FROM users")
    finally:
        await reader.close()


async def test_selfcheck_passes_for_reader(dsn: str) -> None:
    admin = await asyncpg.connect(dsn)
    try:
        await _build_reader(admin)
    finally:
        await admin.close()

    db = Database(dsn)
    # Connect the pool AS nlq_reader by overriding credentials.
    db._dsn = dsn
    reader_pool = await asyncpg.create_pool(
        dsn, user="nlq_reader", password=READER_PASSWORD, min_size=1, max_size=2
    )
    db._pool = reader_pool
    try:
        await run_security_checks(db)  # must not raise
    finally:
        await db.close()
