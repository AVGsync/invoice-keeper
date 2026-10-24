from __future__ import annotations

from datetime import UTC, date, datetime
from zoneinfo import ZoneInfo

import pytest

from nlq.common.dates import resolve_period, today
from nlq.common.errors import BadArgs

TODAY = date(2026, 10, 24)  # Saturday


def test_today_override_parsed() -> None:
    assert today("Europe/Moscow", "2026-10-24") == date(2026, 10, 24)


def test_today_override_invalid() -> None:
    with pytest.raises(BadArgs):
        today("Europe/Moscow", "not-a-date")


def test_today_timezone_crosses_midnight() -> None:
    # 22:30 UTC is already the next day in Moscow (UTC+3); CURRENT_DATE in a UTC
    # container would be a day behind — hence APP_TIMEZONE (§5.1 p.4).
    moscow = datetime(2026, 10, 23, 22, 30, tzinfo=UTC).astimezone(ZoneInfo("Europe/Moscow"))
    assert moscow.date() == date(2026, 10, 24)


def test_period_none() -> None:
    assert resolve_period(None, TODAY) is None


def test_period_none_with_dates_rejected() -> None:
    with pytest.raises(BadArgs):
        resolve_period(None, TODAY, date_from="2026-10-01")


def test_this_week_monday_to_sunday() -> None:
    assert resolve_period("this_week", TODAY) == (date(2026, 10, 19), date(2026, 10, 25))


def test_today_period() -> None:
    assert resolve_period("today", TODAY) == (TODAY, TODAY)


def test_next_7_days() -> None:
    assert resolve_period("next_7_days", TODAY) == (TODAY, date(2026, 10, 30))


def test_this_month() -> None:
    assert resolve_period("this_month", TODAY) == (date(2026, 10, 1), date(2026, 10, 31))


def test_next_month() -> None:
    assert resolve_period("next_month", TODAY) == (date(2026, 11, 1), date(2026, 11, 30))


def test_next_month_year_rollover() -> None:
    assert resolve_period("next_month", date(2026, 12, 15)) == (
        date(2027, 1, 1),
        date(2027, 1, 31),
    )


def test_this_year() -> None:
    assert resolve_period("this_year", TODAY) == (date(2026, 1, 1), date(2026, 12, 31))


def test_leap_year_february() -> None:
    assert resolve_period("this_month", date(2024, 2, 10)) == (
        date(2024, 2, 1),
        date(2024, 2, 29),
    )


def test_custom_requires_both_dates() -> None:
    with pytest.raises(BadArgs):
        resolve_period("custom", TODAY, date_from="2026-11-01")


def test_custom_from_after_to() -> None:
    with pytest.raises(BadArgs):
        resolve_period("custom", TODAY, date_from="2026-11-30", date_to="2026-11-01")


def test_custom_ok() -> None:
    assert resolve_period("custom", TODAY, date_from="2026-11-01", date_to="2026-11-30") == (
        date(2026, 11, 1),
        date(2026, 11, 30),
    )


def test_custom_outside_window_rejected() -> None:
    with pytest.raises(BadArgs):
        resolve_period("custom", TODAY, date_from="2000-01-01", date_to="2000-01-02")
