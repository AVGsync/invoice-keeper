"""Functional tests for the 4 tools against a real Postgres (§12.3, §12.4).

Skipped unless TEST_DATABASE_URL is set. execute() is called directly with a
constructed Actor/Visibility (bypassing HTTP auth), anchored to today=2026-10-24.
"""

from __future__ import annotations

from uuid import UUID

import pytest

from nlq.common.actor import Actor
from nlq.common.tool_specs import (
    FindOccurrencesArgs,
    FindPeopleArgs,
    FindRecurringPaymentsArgs,
    SumOccurrencesArgs,
)
from nlq.mcp_server.policy import resolve_visibility
from nlq.mcp_server.state import AppState
from nlq.mcp_server.tools import (
    find_occurrences,
    find_people,
    find_recurring_payments,
    sum_occurrences,
)

pytestmark = pytest.mark.db

IVAN = UUID("11110000-0000-0000-0000-000000000002")


def _actor(role: str = "user", sub: UUID = IVAN) -> Actor:
    return Actor(sub=sub, role=role, ch="web")


def _vis(state: AppState, actor: Actor):
    return resolve_visibility(actor, state.settings.nlq_user_scope)


async def test_overdue(seeded_state: AppState) -> None:
    actor = _actor("admin")
    res = await find_occurrences.execute(
        FindOccurrencesArgs(status="overdue"), seeded_state, actor, _vis(seeded_state, actor)
    )
    titles = {r["title"] for r in res["rows"]}
    assert res["total"] == 4
    assert {"Хостинг AWS", "Notion", "Яндекс 360"}.issubset(titles)
    assert "Zoom" not in titles  # ended -> not overdue


async def test_sum_usd_this_month(seeded_state: AppState) -> None:
    actor = _actor("admin")
    res = await sum_occurrences.execute(
        SumOccurrencesArgs(status="any", period="this_month", currency="USD"),
        seeded_state, actor, _vis(seeded_state, actor),
    )
    assert set(res["by_currency"]) == {"USD"}
    assert res["by_currency"]["USD"]["total"] == "218.00"


async def test_sum_this_month_all_currencies_separate(seeded_state: AppState) -> None:
    actor = _actor("admin")
    res = await sum_occurrences.execute(
        SumOccurrencesArgs(status="any", period="this_month"),
        seeded_state, actor, _vis(seeded_state, actor),
    )
    assert set(res["by_currency"]) == {"RUB", "USD", "EUR"}
    for v in res["by_currency"].values():
        assert "total" in v
    # There is no cross-currency grand total key.
    assert "total" not in res


async def test_paid_october_boundaries(seeded_state: AppState) -> None:
    actor = _actor("admin")
    res = await sum_occurrences.execute(
        SumOccurrencesArgs(status="paid", period="this_month"),
        seeded_state, actor, _vis(seeded_state, actor),
    )
    # Интернет 2500 RUB (10.10) + GitHub 44 USD (05.10); rent paid 25.09 is out of month.
    assert res["by_currency"]["RUB"]["total"] == "2500.00"
    assert res["by_currency"]["USD"]["total"] == "44.00"


async def test_recurring_stemmer_rent(seeded_state: AppState) -> None:
    actor = _actor("admin")
    res = await find_recurring_payments.execute(
        FindRecurringPaymentsArgs(title_query="аренда"),
        seeded_state, actor, _vis(seeded_state, actor),
    )
    rows = {r["title"]: r for r in res["rows"]}
    assert "Аренда офиса" in rows
    assert rows["Аренда офиса"]["assignee_name"] == "Мария Соколова"


async def test_recurring_eur_over_50(seeded_state: AppState) -> None:
    actor = _actor("admin")
    res = await find_recurring_payments.execute(
        FindRecurringPaymentsArgs(currency="EUR", min_amount=50),
        seeded_state, actor, _vis(seeded_state, actor),
    )
    titles = {r["title"] for r in res["rows"]}
    assert titles == {"Figma", "Notion"}


async def test_people_accountant(seeded_state: AppState) -> None:
    actor = _actor("admin")
    res = await find_people.execute(
        FindPeopleArgs(query="бухгалтер"), seeded_state, actor, _vis(seeded_state, actor)
    )
    names = {r["full_name"] for r in res["rows"]}
    assert "Анна Смирнова" in names


async def test_overdue_me_ivan(seeded_state: AppState) -> None:
    actor = _actor("user", IVAN)
    res = await find_occurrences.execute(
        FindOccurrencesArgs(status="overdue", assignee_id="me"),
        seeded_state, actor, _vis(seeded_state, actor),
    )
    titles = {r["title"] for r in res["rows"]}
    assert titles == {"Хостинг AWS"}


async def test_injection_title_is_data(seeded_state: AppState) -> None:
    actor = _actor("admin")
    res = await find_occurrences.execute(
        FindOccurrencesArgs(status="overdue"), seeded_state, actor, _vis(seeded_state, actor)
    )
    injected = [r for r in res["rows"] if r["title"].startswith("Сервис X")]
    assert len(injected) == 1  # present as a row, treated as data


async def test_like_wildcards_escaped(seeded_state: AppState) -> None:
    actor = _actor("admin")
    # "%" must not act as a wildcard matching everything.
    res = await find_recurring_payments.execute(
        FindRecurringPaymentsArgs(title_query="%"),
        seeded_state, actor, _vis(seeded_state, actor),
    )
    assert res["total"] == 0


async def test_run_rate_monthly_average(seeded_state: AppState) -> None:
    actor = _actor("admin")
    res = await sum_occurrences.execute(
        SumOccurrencesArgs(status="any", basis="run_rate"),
        seeded_state, actor, _vis(seeded_state, actor),
    )
    assert res["basis"] == "run_rate"
    assert res["period"] is None
    # All seed payments are monthly (gap ~30d) or single-occurrence (default x1),
    # so the normalized monthly load is just the sum of active amounts per currency.
    assert res["by_currency"]["RUB"]["total"] == "155500.00"  # 150000 + 3000 + 2500
    assert res["by_currency"]["USD"]["total"] == "174.00"     # 120 + 44 + 10 (Zoom ended)
    assert res["by_currency"]["EUR"]["total"] == "145.00"     # 55 + 60 + 30


async def test_run_rate_currency_filter(seeded_state: AppState) -> None:
    actor = _actor("admin")
    res = await sum_occurrences.execute(
        SumOccurrencesArgs(status="any", basis="run_rate", currency="USD"),
        seeded_state, actor, _vis(seeded_state, actor),
    )
    assert set(res["by_currency"]) == {"USD"}
    assert res["by_currency"]["USD"]["total"] == "174.00"


async def test_coverage_incomplete_next_month(seeded_state: AppState) -> None:
    actor = _actor("admin")
    res = await sum_occurrences.execute(
        SumOccurrencesArgs(status="any", period="next_month"),
        seeded_state, actor, _vis(seeded_state, actor),
    )
    # Сервис X has a single occurrence 18.10, so the earliest per-payment horizon
    # is 2026-10-18 and any window reaching past it is reported incomplete.
    assert res["coverage_until"] == "2026-10-18"
    assert res["coverage_incomplete"] is True


async def test_category_ignored_when_column_absent(seeded_state: AppState) -> None:
    # B has no category column (decision #3): the filter is always unavailable.
    actor = _actor("admin")
    res = await find_recurring_payments.execute(
        FindRecurringPaymentsArgs(category="subscription"),
        seeded_state, actor, _vis(seeded_state, actor),
    )
    assert res["category_unavailable"] is True
    assert res["total"] >= 8  # filter ignored, all active payments returned


async def test_sum_category_ignored_when_column_absent(seeded_state: AppState) -> None:
    # B has no category column: sum_occurrences reports it unavailable, not empty.
    actor = _actor("admin")
    res = await sum_occurrences.execute(
        SumOccurrencesArgs(status="any", category="infra"),
        seeded_state, actor, _vis(seeded_state, actor),
    )
    assert res["category_unavailable"] is True
    # All currencies still present because the category filter is dropped.
    assert set(res["by_currency"]) == {"RUB", "USD", "EUR"}


async def test_occurrence_assignee_always_present(seeded_state: AppState) -> None:
    # Every period carries a NOT NULL user_id, so assignee_name is always set.
    actor = _actor("admin")
    res = await find_occurrences.execute(
        FindOccurrencesArgs(status="any", title_query="Интернет"),
        seeded_state, actor, _vis(seeded_state, actor),
    )
    internet = [r for r in res["rows"] if r["title"] == "Интернет"]
    assert internet and internet[0]["assignee_name"] == "Анна Смирнова"
