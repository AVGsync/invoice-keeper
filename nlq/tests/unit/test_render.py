"""Block/notes/followups/template building from tool results (§7.7, §7.9)."""

from __future__ import annotations

from typing import Any

from nlq.agent.render import (
    build_blocks,
    build_followups,
    build_notes,
    template_text,
)
from nlq.agent.types import ToolResult


def _result(name: str, payload: dict[str, Any]) -> ToolResult:
    payload = {"ok": True, "tool": name, **payload}
    return ToolResult(call_id="c", name=name, payload=payload)


def test_occurrence_block_status_derivation() -> None:
    res = _result(
        "find_occurrences",
        {
            "total": 2,
            "truncated": False,
            "rows": [
                {"title": "AWS", "amount": "120.00", "currency": "USD", "display": "120,00 $",
                 "due_date": "2026-10-20", "days_overdue": 4, "is_paid": False,
                 "occurrence_id": "o1", "payment_id": "p1"},
                {"title": "GitHub", "amount": "44.00", "currency": "USD", "display": "44,00 $",
                 "due_date": "2026-10-05", "days_overdue": None, "is_paid": True,
                 "occurrence_id": "o2", "payment_id": "p2"},
            ],
        },
    )
    blocks = build_blocks([res])
    assert blocks[0]["type"] == "occurrence_list"
    assert blocks[0]["items"][0]["status"] == "overdue"
    assert blocks[0]["items"][1]["status"] == "paid"


def test_currency_totals_block_no_grand_total() -> None:
    res = _result(
        "sum_occurrences",
        {
            "by_currency": {"USD": {"total": "218.00", "paid": "44.00", "unpaid": "174.00",
                                    "count": 4, "display": "218,00 $"}},
            "groups": [],
            "period": {"from": "2026-10-01", "to": "2026-10-31"},
            "coverage_until": "2026-12-25",
            "coverage_incomplete": False,
        },
    )
    block = build_blocks([res])[0]
    assert block["type"] == "currency_totals"
    assert "всего" not in block
    assert set(block["by_currency"]) == {"USD"}


def test_currency_totals_template_grammar_run_rate() -> None:
    # Падеж «в долларах» читается верно: «нагрузка — в долларах», не «— долларах».
    res = _result(
        "sum_occurrences",
        {
            "basis": "run_rate",
            "by_currency": {"USD": {"total": "218.00", "paid": "0.00", "unpaid": "218.00",
                                    "count": 4, "display": "218,00 $"}},
            "groups": [],
            "period": None,
            "coverage_until": None,
            "coverage_incomplete": False,
        },
    )
    blocks = build_blocks([res])
    text = template_text(blocks)
    assert "в долларах" in text
    assert "— долларах" not in text


def test_currency_totals_template_grammar_period() -> None:
    res = _result(
        "sum_occurrences",
        {
            "by_currency": {"RUB": {"total": "153000.00", "paid": "0.00", "unpaid": "153000.00",
                                    "count": 2, "display": "153 000,00 ₽"}},
            "groups": [],
            "period": {"from": "2026-10-01", "to": "2026-10-31"},
            "coverage_until": "2026-12-25",
            "coverage_incomplete": False,
        },
    )
    text = template_text(build_blocks([res]))
    assert "в рублях" in text


def test_people_block_suppressed_when_followed_by_assignee_lookup() -> None:
    people = _result("find_people", {"rows": [{"user_id": "u1", "full_name": "Иван"}]})
    payments = _result("find_recurring_payments", {"total": 1, "truncated": False, "rows": []})
    blocks = build_blocks([people, payments])
    types = {b["type"] for b in blocks}
    assert "people_list" not in types
    assert "payment_list" in types


def test_people_block_kept_when_standalone() -> None:
    people = _result("find_people", {"rows": [{"user_id": "u1", "full_name": "Анна"}]})
    blocks = build_blocks([people])
    assert blocks[0]["type"] == "people_list"


def test_notes_currency_and_subscriptions() -> None:
    res = _result("find_recurring_payments", {"total": 2, "truncated": False, "rows": []})
    notes = build_notes([res], {"c": {"currency": "EUR", "min_amount": 50}}, "подписки дороже 50 евро", "team")
    assert any("валюты" in n.lower() for n in notes)
    assert any("категор" in n.lower() for n in notes)


def test_notes_truncated_and_own() -> None:
    res = _result("find_occurrences", {"total": 34, "truncated": True,
                                        "rows": [{"title": "x"} for _ in range(20)]})
    notes = build_notes([res], {}, "что просрочено?", "own")
    assert any("20" in n and "34" in n for n in notes)
    assert any("только ваши" in n for n in notes)


def test_followups_by_last_tool() -> None:
    res = _result("find_occurrences", {"total": 1, "truncated": False, "rows": []})
    fu = build_followups([res])
    assert 1 <= len(fu) <= 3


def test_template_text_from_blocks() -> None:
    res = _result("find_occurrences", {"total": 3, "truncated": False, "rows": []})
    blocks = build_blocks([res])
    text = template_text(blocks)
    assert "3" in text
