from __future__ import annotations

from decimal import Decimal

import pytest

from nlq.common import money
from nlq.common.errors import BadArgs

NBSP = " "


def test_amount_string_two_places() -> None:
    assert money.to_amount_string(Decimal("150000")) == "150000.00"
    assert money.to_amount_string(Decimal("120.5")) == "120.50"


def test_display_rub_grouped() -> None:
    assert money.display(Decimal("150000"), "RUB") == f"150{NBSP}000,00{NBSP}₽"


def test_display_usd_small() -> None:
    assert money.display(Decimal("120"), "USD") == f"120,00{NBSP}$"


def test_display_eur() -> None:
    assert money.display(Decimal("55"), "EUR") == f"55,00{NBSP}€"


def test_display_millions() -> None:
    assert money.display(Decimal("1234567.89"), "RUB") == f"1{NBSP}234{NBSP}567,89{NBSP}₽"


def test_parse_amount_ok() -> None:
    assert money.parse_amount(50, field="min_amount") == Decimal("50")
    assert money.parse_amount(50.5, field="min_amount") == Decimal("50.5")


def test_parse_amount_negative_rejected() -> None:
    with pytest.raises(BadArgs):
        money.parse_amount(-1, field="min_amount")
