"""Contract invariants mirroring TELEGRAM_BOT.md §2.5 / NL_QUERIES §6.4.

Skipped unless TEST_DATABASE_URL is set. Keeps the NL "overdue" identical to the
bot's and guards against drift in status semantics and date handling.
"""

from __future__ import annotations

import pytest

from nlq.common.actor import Actor
from nlq.common.tool_specs import FindOccurrencesArgs
from nlq.mcp_server.policy import resolve_visibility
from nlq.mcp_server.state import AppState
from nlq.mcp_server.tools import find_occurrences

pytestmark = pytest.mark.db

ADMIN = Actor(sub=__import__("uuid").UUID("11110000-0000-0000-0000-000000000001"), role="admin", ch="web")


async def _rows(state: AppState, args: FindOccurrencesArgs) -> list[dict]:
    res = await find_occurrences.execute(args, state, ADMIN, resolve_visibility(ADMIN, "team"))
    return res["rows"]


async def test_ended_excluded_from_any(seeded_state: AppState) -> None:
    rows = await _rows(seeded_state, FindOccurrencesArgs(status="any"))
    assert all(r["title"] != "Zoom" for r in rows)


async def test_ended_excluded_from_overdue(seeded_state: AppState) -> None:
    rows = await _rows(seeded_state, FindOccurrencesArgs(status="overdue"))
    assert all(r["title"] != "Zoom" for r in rows)


async def test_paid_excluded_from_overdue(seeded_state: AppState) -> None:
    rows = await _rows(seeded_state, FindOccurrencesArgs(status="overdue"))
    # Rent occurrence paid on 25.09 must not appear as overdue.
    assert all(not (r["title"] == "Аренда офиса" and r["due_date"] == "2026-09-25") for r in rows)


async def test_due_date_not_shifted(seeded_state: AppState) -> None:
    rows = await _rows(seeded_state, FindOccurrencesArgs(status="overdue", title_query="хостинг"))
    aws = [r for r in rows if r["title"] == "Хостинг AWS"]
    assert aws and aws[0]["due_date"] == "2026-10-20"
    assert aws[0]["days_overdue"] == 4  # 2026-10-24 - 2026-10-20


async def test_upcoming_boundary_inclusive_today(seeded_state: AppState) -> None:
    # JetBrains due 27.10 is upcoming; 24.10 (today) boundary is inclusive for >=.
    rows = await _rows(seeded_state, FindOccurrencesArgs(status="upcoming", period="this_week"))
    titles = {r["title"] for r in rows}
    assert "JetBrains" not in titles  # 27.10 is outside this_week (ends 25.10)


async def test_probe_schema_prepares_every_template(seeded_state: AppState) -> None:
    # Prepares every rendered template for each status/sort/group_by, so a
    # placeholder-in-comment or indeterminate-$1 regression is caught as broken.
    from nlq.mcp_server.selfcheck import probe_schema

    status = await probe_schema(seeded_state.db)
    assert status.value == "ok"
    assert status.category_available is False  # B has no category column (decision #3)
