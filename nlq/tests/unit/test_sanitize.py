from __future__ import annotations

from nlq.common.sanitize import TITLE_MAX_LEN, sanitize


def test_empty() -> None:
    assert sanitize(None) == ""
    assert sanitize("") == ""


def test_plain_passthrough() -> None:
    assert sanitize("Аренда офиса") == "Аренда офиса"


def test_strips_bidi_override() -> None:
    # Right-to-left override used to disguise text.
    assert sanitize("abc‮def") == "abcdef"


def test_strips_isolates() -> None:
    assert sanitize("a⁦b⁩c") == "abc"


def test_strips_zero_width() -> None:
    assert sanitize("a​b‍c﻿d") == "abcd"


def test_strips_control_chars() -> None:
    assert sanitize("a\x00b\x07c") == "abc"


def test_collapses_whitespace() -> None:
    assert sanitize("a\t\nb   c") == "a b c"


def test_truncates_to_max() -> None:
    out = sanitize("x" * 200)
    assert len(out) == TITLE_MAX_LEN
