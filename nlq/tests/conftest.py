"""Shared fixtures. DB-backed fixtures skip cleanly when TEST_DATABASE_URL is unset."""

from __future__ import annotations

import base64
import os
from collections.abc import AsyncIterator
from pathlib import Path

import asyncpg
import pytest
import pytest_asyncio
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey

from nlq.config import Settings
from nlq.mcp_server.db import Database
from nlq.mcp_server.state import SCHEMA_OK, AppState, SchemaStatus, set_state

FIXTURES = Path(__file__).parent / "fixtures"
DEV_SCHEMA = "nlq_test"

# Minimal public tables so tests do not depend on backend/migrations being run.
_PUBLIC_DDL = """
CREATE TABLE IF NOT EXISTS personas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    full_name TEXT NOT NULL,
    login TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    persona_id UUID REFERENCES personas(id) ON DELETE SET NULL
);
"""


def require_test_database(dsn: str) -> None:
    """Refuse to run destructive DB fixtures on a non-test database.

    These fixtures TRUNCATE public.users/personas and reset the nlq_reader role,
    which would wreck a real database. Unless the database name ends with
    ``_test`` the tests skip, unless NLQ_ALLOW_DESTRUCTIVE_TESTS is set.
    """
    if os.environ.get("NLQ_ALLOW_DESTRUCTIVE_TESTS"):
        return
    name = dsn.rsplit("/", 1)[-1].split("?", 1)[0]
    if not name.endswith("_test"):
        pytest.skip(
            f"refusing destructive DB tests on {name!r}: point TEST_DATABASE_URL at a "
            "database whose name ends with '_test', or set NLQ_ALLOW_DESTRUCTIVE_TESTS=1"
        )


def _test_dsn() -> str:
    dsn = os.environ.get("TEST_DATABASE_URL")
    if not dsn:
        pytest.skip("TEST_DATABASE_URL not set")
    require_test_database(dsn)
    return dsn


@pytest.fixture(scope="session")
def keypair() -> tuple[Ed25519PrivateKey, str]:
    priv = Ed25519PrivateKey.generate()
    pub_raw = priv.public_key().public_bytes(
        serialization.Encoding.Raw, serialization.PublicFormat.Raw
    )
    return priv, base64.b64encode(pub_raw).decode("ascii")


async def _setup_database(conn: asyncpg.Connection) -> None:
    await conn.execute(f'DROP SCHEMA IF EXISTS "{DEV_SCHEMA}" CASCADE')
    await conn.execute(f'CREATE SCHEMA "{DEV_SCHEMA}"')
    await conn.execute(_PUBLIC_DDL)
    await conn.execute("TRUNCATE public.users, public.personas CASCADE")
    await conn.execute(f'SET search_path TO "{DEV_SCHEMA}", public')
    await conn.execute((Path(__file__).parent.parent / "db" / "dev_b_tables.sql").read_text())
    await conn.execute((FIXTURES / "seed.sql").read_text())
    await conn.execute("RESET search_path")


@pytest_asyncio.fixture
async def seeded_state(keypair: tuple[Ed25519PrivateKey, str]) -> AsyncIterator[AppState]:
    dsn = _test_dsn()
    admin = await asyncpg.connect(dsn)
    try:
        await _setup_database(admin)
    finally:
        await admin.close()

    _, pub_b64 = keypair
    settings = Settings(
        APP_ENV="local",
        APP_TIMEZONE="Europe/Moscow",
        NLQ_TODAY="2026-10-24",
        NLQ_ACTOR_PUBLIC_KEY=pub_b64,
        NLQ_DB_DSN=dsn,
        NLQ_DEV_SCHEMA=DEV_SCHEMA,
        NLQ_USER_SCOPE="team",
    )
    db = Database(dsn, dev_schema=DEV_SCHEMA)
    await db.connect()
    state = AppState(
        settings=settings, db=db, public_key=None,  # type: ignore[arg-type]
        schema=SchemaStatus(SCHEMA_OK, None),
    )
    set_state(state)
    try:
        yield state
    finally:
        await db.close()
