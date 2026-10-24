"""Code-controlled SQL fragments selected by enum.

User-supplied text never reaches SQL text (§5.1 p.7): the model picks a status
or sort *enum*, and we look up a fixed fragment. ``escape_like`` escapes the
LIKE metacharacters in the (separately parameterised) search string.
"""

from __future__ import annotations

from typing import Final

# status -> predicate over the `base` CTE, using $1 = today (section 5.1 p.6).
# Every value is fully parenthesised so splicing it into a WHERE that ANDs the
# other filters can never be reparsed by precedence (AND binds tighter than OR):
# `any` must stay (paid OR not-ended), not paid OR (not-ended AND filters).
STATUS_SQL: Final[dict[str, str]] = {
    "overdue": "(NOT b.is_paid AND NOT b.is_ended AND b.due_date < $1)",
    "upcoming": "(NOT b.is_paid AND NOT b.is_ended AND b.due_date >= $1)",
    "unpaid": "(NOT b.is_paid AND NOT b.is_ended)",
    "paid": "(b.is_paid)",
    "any": "(b.is_paid OR NOT b.is_ended)",
}


def category_col(available: bool) -> str:
    """SELECT expression for the optional B category column (decision #3).

    B has no ``recurrent_payments.category`` column, so ``available`` is always
    False and we select ``NULL::text``: the adapter still parses and the tool
    reports the category filter as unavailable instead of failing. The hook is
    kept in case B ships a category column later.
    """
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
    """Escape backslash and LIKE wildcards for ILIKE ... ESCAPE '\\'."""
    return value.replace("\\", "\\\\").replace("%", "\\%").replace("_", "\\_")
