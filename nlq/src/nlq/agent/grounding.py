"""Text cleaning and the grounding check."""

from __future__ import annotations

import re
from datetime import date
from decimal import Decimal, InvalidOperation
from typing import Any

MAX_TEXT_LEN = 1200

_MD_IMAGE = re.compile(r"!\[[^\]]*\]\([^)]*\)")
_MD_LINK = re.compile(r"\[([^\]]*)\]\([^)]*\)")
_URL = re.compile(r"(?:https?://|www\.)\S+", re.IGNORECASE)
_HTML_TAG = re.compile(r"</?[a-zA-Z][^>]*>")
_MARKUP_CHARS = re.compile(r"[*_#`~]")
_BIDI_ZW = re.compile(r"[​-‏‪-‮⁦-⁩﻿]")

_DATE = re.compile(r"\b(\d{1,2})\.(\d{1,2})(?:\.(\d{2,4}))?\b")
_NUMBER = re.compile(r"\d[\d\s  ]*(?:[.,]\d+)?")


# Служебные метки памяти диалога: модель не должна показывать их пользователю.
_MEMORY_MARKER = re.compile(r"\s*\[(?:Данные этого ответа|Инструменты:)[^\n]*\]")


def strip_memory_markers(text: str) -> str:
    return _MEMORY_MARKER.sub("", text).strip()


def clean_text(text: str | None) -> str:
    if not text:
        return ""
    out = strip_memory_markers(text)
    out = _MD_IMAGE.sub(" ", out)
    out = _MD_LINK.sub(r"\1", out)
    out = _URL.sub(" ", out)
    out = _HTML_TAG.sub(" ", out)
    out = _MARKUP_CHARS.sub("", out)
    out = _BIDI_ZW.sub("", out)
    out = "".join(ch if ch >= " " or ch == "\n" else " " for ch in out)
    out = re.sub(r"[ \t]+", " ", out)
    out = re.sub(r"\n{3,}", "\n\n", out).strip()
    if len(out) > MAX_TEXT_LEN:
        out = out[:MAX_TEXT_LEN].rstrip()
    return out


def _normalize_number(raw: str) -> str | None:
    """Canonicalise a matched number: drop separators, 150 000,00 -> 150000."""
    cleaned = raw.replace(" ", "").replace(" ", "").replace(" ", "")
    cleaned = cleaned.replace(",", ".")
    cleaned = cleaned.strip(".")
    if not cleaned or not any(c.isdigit() for c in cleaned):
        return None
    try:
        value = Decimal(cleaned)
    except InvalidOperation:
        return None
    normalized = value.normalize()
    text = format(normalized, "f")
    if "." in text:
        text = text.rstrip("0").rstrip(".")
    return text or "0"


def _extract_numbers(text: str) -> list[str]:
    found: list[str] = []
    for match in _NUMBER.finditer(text):
        norm = _normalize_number(match.group(0))
        if norm is not None:
            found.append(norm)
    return found


def _extract_dates_and_rest(text: str) -> tuple[list[str], str]:
    """Pull dotted-date components out of the text; return (components, rest)."""
    components: list[str] = []

    def _replace(m: re.Match[str]) -> str:
        for part in m.groups():
            if part is not None:
                components.append(str(int(part)))
        return " "

    rest = _DATE.sub(_replace, text)
    return components, rest


def _add(allowed: set[str], value: Any) -> None:
    if value is None:
        return
    if isinstance(value, bool):
        return
    if isinstance(value, int):
        allowed.add(str(value))
        return
    norm = _normalize_number(str(value))
    if norm is not None:
        allowed.add(norm)


def _add_date_components(allowed: set[str], iso: str) -> None:
    try:
        d = date.fromisoformat(iso[:10])
    except ValueError:
        return
    for part in (d.year, d.month, d.day):
        allowed.add(str(part))


# Numbers in the text may only come from these numeric keys.
_NUMERIC_KEYS = frozenset(
    {
        "amount",
        "display",
        "total",
        "paid",
        "unpaid",
        "count",
        "total_count",
        "cnt",
        "days_overdue",
        "active_payments",
        "numbers",
        "a_total",
        "b_total",
        "delta",
        "delta_pct",
        "a_count",
        "b_count",
        "a_display",
        "b_display",
        "delta_display",
        "overdue_count",
        "users_count",
        "paid_count_12m",
        "interval",
    }
)
_DATE_KEYS = frozenset(
    {"due_date", "next_due", "coverage_until", "today", "from", "to", "start_date", "last_paid_date"}
)
_MONTH_KEYS = frozenset({"month"})


def _harvest(allowed: set[str], value: Any, key: str | None = None) -> None:
    """Walk a tool payload and collect numbers/dates from whitelisted keys only."""
    if isinstance(value, dict):
        for k, v in value.items():
            _harvest(allowed, v, k)
    elif isinstance(value, list):
        for item in value:
            _harvest(allowed, item, key)
    elif key in _DATE_KEYS and isinstance(value, str):
        _add_date_components(allowed, value)
    elif key in _MONTH_KEYS and isinstance(value, str):
        m = re.match(r"^(\d{4})-(\d{1,2})$", value)
        if m:
            allowed.add(str(int(m.group(1))))
            allowed.add(str(int(m.group(2))))
    elif key in _NUMERIC_KEYS:
        if isinstance(value, (int, float)):
            _add(allowed, value)
        elif isinstance(value, str):
            for match in _NUMBER.finditer(value):
                _add(allowed, match.group(0))


def build_allowed_numbers(
    tool_payloads: list[dict[str, Any]], question: str, today: date
) -> set[str]:
    allowed: set[str] = set()
    for payload in tool_payloads:
        _harvest(allowed, payload)
    for n in _extract_numbers(question):
        allowed.add(n)
    for part in (today.year, today.month, today.day):
        allowed.add(str(part))
    return allowed


def is_grounded(text: str, allowed: set[str]) -> bool:
    date_components, rest = _extract_dates_and_rest(text)
    if any(c not in allowed for c in date_components):
        return False
    return all(n in allowed for n in _extract_numbers(rest))
