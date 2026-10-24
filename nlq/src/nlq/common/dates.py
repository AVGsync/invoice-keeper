"""Server-side "today" and period resolution."""

from __future__ import annotations

import calendar
from datetime import date, datetime, timedelta
from typing import Final, Literal
from zoneinfo import ZoneInfo

from .errors import BadArgs

Period = Literal[
    "today",
    "this_week",
    "last_week",
    "next_7_days",
    "this_month",
    "last_month",
    "next_month",
    "this_year",
    "custom",
]

PERIODS: Final = frozenset(
    {
        "today",
        "this_week",
        "last_week",
        "next_7_days",
        "this_month",
        "last_month",
        "next_month",
        "this_year",
        "custom",
    }
)

_MAX_DELTA_DAYS: Final = 366 * 2


def today(tz: str, override: str | None = None) -> date:
    """Current date in ``tz``."""
    if override:
        try:
            return date.fromisoformat(override)
        except ValueError as exc:
            raise BadArgs("NLQ_TODAY must be YYYY-MM-DD") from exc
    return datetime.now(ZoneInfo(tz)).date()


def parse_date(value: str, *, field: str) -> date:
    try:
        return date.fromisoformat(value)
    except ValueError as exc:
        raise BadArgs(f"{field} must be YYYY-MM-DD") from exc


def _month_last_day(year: int, month: int) -> int:
    return calendar.monthrange(year, month)[1]


def _first_of_next_month(d: date) -> date:
    if d.month == 12:
        return date(d.year + 1, 1, 1)
    return date(d.year, d.month + 1, 1)


def resolve_period(
    period: str | None,
    today_: date,
    date_from: str | None = None,
    date_to: str | None = None,
) -> tuple[date, date] | None:
    """Expand a period enum into inclusive (from, to) bounds."""
    if period is None:
        if date_from is not None or date_to is not None:
            raise BadArgs("date_from/date_to are only allowed with period=custom")
        return None

    if period not in PERIODS:
        raise BadArgs("unknown period")

    if period == "today":
        return today_, today_
    if period == "this_week":
        monday = today_ - timedelta(days=today_.weekday())
        return monday, monday + timedelta(days=6)
    if period == "last_week":
        monday = today_ - timedelta(days=today_.weekday()) - timedelta(days=7)
        return monday, monday + timedelta(days=6)
    if period == "next_7_days":
        return today_, today_ + timedelta(days=6)
    if period == "this_month":
        return (
            date(today_.year, today_.month, 1),
            date(today_.year, today_.month, _month_last_day(today_.year, today_.month)),
        )
    if period == "last_month":
        end = date(today_.year, today_.month, 1) - timedelta(days=1)
        return date(end.year, end.month, 1), end
    if period == "next_month":
        start = _first_of_next_month(today_)
        return start, date(start.year, start.month, _month_last_day(start.year, start.month))
    if period == "this_year":
        return date(today_.year, 1, 1), date(today_.year, 12, 31)

    if date_from is None or date_to is None:
        raise BadArgs("period=custom requires both date_from and date_to")
    d_from = parse_date(date_from, field="date_from")
    d_to = parse_date(date_to, field="date_to")
    if d_from > d_to:
        raise BadArgs("date_from must be <= date_to")
    _check_window(d_from, today_)
    _check_window(d_to, today_)
    return d_from, d_to


def _check_window(d: date, today_: date) -> None:
    if abs((d - today_).days) > _MAX_DELTA_DAYS:
        raise BadArgs("date is outside the allowed window (today +/- 2 years)")
