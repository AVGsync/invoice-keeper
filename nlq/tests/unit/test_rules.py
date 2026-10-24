"""Rules router (§7.8)."""

from __future__ import annotations

import json

import pytest

from nlq.agent.rules import route


def _single(question: str) -> tuple[str, dict]:
    calls = route(question)
    assert len(calls) == 1
    return calls[0].name, json.loads(calls[0].arguments)


def test_overdue() -> None:
    name, args = _single("что просрочено?")
    assert name == "find_occurrences"
    assert args["status"] == "overdue"


def test_today() -> None:
    name, args = _single("какие платежи сегодня?")
    assert name == "find_occurrences"
    assert args == {"status": "unpaid", "period": "today"}


def test_this_week() -> None:
    _, args = _single("что платить на этой неделе?")
    assert args == {"status": "upcoming", "period": "this_week"}


def test_sum_in_dollars() -> None:
    # "в месяц" without a named month -> normalized monthly average (decision #2).
    name, args = _single("сколько платим в долларах в месяц?")
    assert name == "sum_occurrences"
    assert args["currency"] == "USD"
    assert args["basis"] == "run_rate"
    assert "period" not in args


def test_sum_generic() -> None:
    name, args = _single("сколько платим в месяц?")
    assert name == "sum_occurrences"
    assert args["basis"] == "run_rate"
    assert "currency" not in args


def test_sum_this_month_named() -> None:
    name, args = _single("сколько в этом месяце платим?")
    assert name == "sum_occurrences"
    assert args["period"] == "this_month"
    assert "basis" not in args or args["basis"] == "period_sum"


def test_sum_already_paid_october() -> None:
    name, args = _single("сколько мы уже оплатили?")
    assert name == "sum_occurrences"
    assert args["status"] == "paid"


def test_who_is_responsible() -> None:
    name, args = _single("кто отвечает за аренду?")
    assert name == "find_recurring_payments"
    assert "аренда" in args["title_query"].lower() or "аренду" in args["title_query"].lower()


def test_dearer_than() -> None:
    name, args = _single("покажи подписки дороже 50 евро")
    assert name == "find_recurring_payments"
    assert args["currency"] == "EUR"
    assert args["min_amount"] == 50


def test_yo_normalization() -> None:
    # "ё" -> "е" so "платёж" matches.
    name, _ = _single("сколько платежей")
    assert name == "sum_occurrences"


def test_out_of_scope_help() -> None:
    calls = route("какая погода в москве?")
    assert len(calls) == 1
    assert calls[0].name == "reply_without_data"
    assert json.loads(calls[0].arguments)["reason"] == "out_of_scope"


@pytest.mark.parametrize("q", ["что просрочено", "ЧТО ПРОСРОЧЕНО?!"])
def test_case_insensitive(q: str) -> None:
    name, args = _single(q)
    assert name == "find_occurrences"
    assert args["status"] == "overdue"
