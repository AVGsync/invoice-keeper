"""Sanitise strings coming out of the database."""

from __future__ import annotations

from typing import Final

TITLE_MAX_LEN: Final = 120

_BIDI: Final = frozenset(range(0x202A, 0x202F)) | frozenset(range(0x2066, 0x206A))
_ZERO_WIDTH: Final = frozenset({0x200B, 0x200C, 0x200D, 0xFEFF})


def sanitize(value: str | None, *, max_len: int = TITLE_MAX_LEN) -> str:
    """Remove control/bidi/zero-width characters, collapse whitespace, trim."""
    if not value:
        return ""
    out: list[str] = []
    for ch in value:
        cp = ord(ch)
        if cp in _BIDI or cp in _ZERO_WIDTH:
            continue
        if cp < 0x20 or 0x7F <= cp <= 0x9F:
            if ch in ("\t", "\n", "\r"):
                out.append(" ")
            continue
        out.append(ch)
    cleaned = " ".join("".join(out).split())
    if len(cleaned) > max_len:
        cleaned = cleaned[:max_len].rstrip()
    return cleaned
