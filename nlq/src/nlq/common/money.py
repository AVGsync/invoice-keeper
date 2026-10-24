"""Money handling: Decimal in, canonical string + Russian display out."""

from __future__ import annotations

from decimal import ROUND_HALF_UP, Decimal, InvalidOperation
from typing import Final

from .errors import BadArgs

CURRENCIES: Final = frozenset({"RUB", "USD", "EUR"})
_SYMBOLS: Final = {"RUB": "₽", "USD": "$", "EUR": "€"}
_TWO_PLACES: Final = Decimal("0.01")


def to_amount_string(value: Decimal) -> str:
    """Canonical amount: fixed two decimals, e.g. ``150000.00``."""
    return str(value.quantize(_TWO_PLACES, rounding=ROUND_HALF_UP))


def display(value: Decimal, currency: str) -> str:
    """Human display, Russian convention: ``150 000,00 ₽``."""
    q = value.quantize(_TWO_PLACES, rounding=ROUND_HALF_UP)
    sign = "-" if q < 0 else ""
    digits = f"{abs(q):.2f}"
    int_part, frac_part = digits.split(".")
    groups: list[str] = []
    while len(int_part) > 3:
        groups.insert(0, int_part[-3:])
        int_part = int_part[:-3]
    groups.insert(0, int_part)
    grouped = " ".join(groups)
    suffix = _SYMBOLS.get(currency, currency)
    return f"{sign}{grouped},{frac_part} {suffix}"


def parse_amount(value: object, *, field: str) -> Decimal:
    """Validate a model-supplied amount filter: finite and >= 0."""
    try:
        amount = Decimal(str(value))
    except (InvalidOperation, ValueError, TypeError) as exc:
        raise BadArgs(f"{field} is not a number") from exc
    if not amount.is_finite():
        raise BadArgs(f"{field} must be finite")
    if amount < 0:
        raise BadArgs(f"{field} must be >= 0")
    return amount
