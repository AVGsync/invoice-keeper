"""Code-controlled SQL fragments selected by enum."""

from __future__ import annotations

from typing import Final

STATUS_SQL: Final[dict[str, str]] = {
    "overdue": "(NOT b.is_paid AND NOT b.is_ended AND b.due_date < $1)",
    "upcoming": "(NOT b.is_paid AND NOT b.is_ended AND b.due_date >= $1)",
    "unpaid": "(NOT b.is_paid AND NOT b.is_ended)",
    "paid": "(b.is_paid)",
    "any": "(b.is_paid OR NOT b.is_ended)",
}


def category_col(available: bool) -> str:
    """SELECT expression for the optional B category column."""
    return "rp.category" if available else "NULL::text"

OCCURRENCE_ORDER_SQL: Final[dict[str, str]] = {
    "due_date": "b.due_date, b.title",
    "amount_desc": "b.amount DESC, b.due_date",
}
DEFAULT_OCCURRENCE_ORDER: Final = "due_date"

RECURRING_ORDER_SQL: Final[dict[str, str]] = {
    "title": "rp.description",
    "amount_desc": "rp.amount DESC, rp.description",
    "next_due": "nx.next_due ASC NULLS LAST, rp.description",
}
DEFAULT_RECURRING_ORDER: Final = "title"


def escape_like(value: str) -> str:
    """Escape backslash and LIKE wildcards for ILIKE ..."""
    return value.replace("\\", "\\\\").replace("%", "\\%").replace("_", "\\_")
