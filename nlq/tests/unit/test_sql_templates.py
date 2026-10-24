"""Rendered SQL templates are well-formed (regression for the comment-placeholder
and parameter bugs: section 5.1).

These run without a database: they check that str.format substitution leaves no
stray placeholder, never splices a multi-line fragment into a comment line, and
that $1 and the status fragment are referenced safely in the final WHERE.
"""

from __future__ import annotations

import pytest

from nlq.mcp_server.fragments import STATUS_SQL
from nlq.mcp_server.selfcheck import _rendered_templates


@pytest.mark.parametrize("category_available", [True, False])
def test_no_leftover_placeholders(category_available: bool) -> None:
    for sql in _rendered_templates(category_available):
        assert "{" not in sql and "}" not in sql, sql


# Markers that only appear inside the occurrences_base fragment: if any of
# them lands on a comment line, a multi-line fragment was spliced into a comment.
_BASE_LEAK_MARKERS = ("FROM payment_occurrences", "JOIN recurring_payments")


@pytest.mark.parametrize("category_available", [True, False])
def test_no_base_leak_into_comments(category_available: bool) -> None:
    for sql in _rendered_templates(category_available):
        for line in sql.splitlines():
            stripped = line.strip()
            if stripped.startswith("--"):
                assert not any(m in line for m in _BASE_LEAK_MARKERS), line
        # The first non-comment line must open the statement, not dangle a
        # fragment that leaked out of a broken comment substitution.
        first_sql = next(
            s for line in sql.splitlines() if (s := line.strip()) and not s.startswith("--")
        )
        assert first_sql.upper().startswith(("WITH", "SELECT")), first_sql


def test_status_fragments_are_parenthesised() -> None:
    # Unparenthesised `any` (paid OR not-ended) would be reparsed by precedence
    # once ANDed with the filters, leaking paid rows outside the period/scope.
    for value in STATUS_SQL.values():
        assert value.startswith("(") and value.endswith(")"), value
    assert STATUS_SQL["any"] == "(b.is_paid OR NOT b.is_ended)"


def test_find_occurrences_references_param_one() -> None:
    # $1 must appear even for statuses whose predicate does not use it (unpaid/
    # paid/any), or Postgres rejects the statement at Parse time.
    from nlq.mcp_server.db import load_sql
    from nlq.mcp_server.fragments import category_col

    base = load_sql("occurrences_base.sql").format(category_col=category_col(False))
    for status in STATUS_SQL.values():
        sql = load_sql("find_occurrences.sql").format(
            base=base, status_sql=status, order_sql="b.due_date, b.title"
        )
        assert "$1" in sql.split("FROM base b", 1)[1]
