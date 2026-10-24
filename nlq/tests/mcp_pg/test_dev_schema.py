"""Guard: the dev-schema mirror (db/dev_b_tables.sql) must match B's real public
schema, so tests cannot drift away from what the backend migrations create.

Skipped unless TEST_DATABASE_URL is set AND the backend migrations have been
applied to public (CI runs `migrate up` first). When public has no payment
tables yet, there is nothing to compare against and the test skips.
"""

from __future__ import annotations

import pytest

from nlq.mcp_server.state import AppState

pytestmark = pytest.mark.db

DEV_SCHEMA = "nlq_test"
_TABLES = ("recurrent_payments", "user_recurrent_payments", "payments")

_COLS_SQL = """
SELECT column_name, data_type
FROM information_schema.columns
WHERE table_schema = $1 AND table_name = $2
ORDER BY column_name
"""


async def test_dev_schema_matches_public(seeded_state: AppState) -> None:
    db = seeded_state.db
    for table in _TABLES:
        public_cols = await db.ro_query(_COLS_SQL, "public", table)
        if not public_cols:
            pytest.skip(f"public.{table} absent: run backend/migrations before this test")
        dev_cols = await db.ro_query(_COLS_SQL, DEV_SCHEMA, table)
        public_map = {r["column_name"]: r["data_type"] for r in public_cols}
        dev_map = {r["column_name"]: r["data_type"] for r in dev_cols}
        assert dev_map == public_map, (
            f"{table}: dev schema drifted from public "
            f"(dev-only={set(dev_map) - set(public_map)}, "
            f"public-only={set(public_map) - set(dev_map)})"
        )
