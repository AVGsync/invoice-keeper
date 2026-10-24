"""Sanitise strings coming out of the database.

Titles and names are *data, not instructions* (§8.1). We strip control
characters, bidi overrides and zero-width characters (which can smuggle hidden
text past a reader or the model) and trim to a bounded length.
"""

from __future__ import annotations

from typing import Final

TITLE_MAX_LEN: Final = 120

# Bidi controls: U+202A-202E (embeddings/overrides) and U+2066-2069 (isolates).
_BIDI: Final = frozenset(range(0x202A, 0x202F)) | frozenset(range(0x2066, 0x206A))
# Zero-width: ZWSP, ZWNJ, ZWJ, and BOM / zero-width no-break space.
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
        # Drop C0/C1 control characters except tab/newline (normalised below).
        if cp < 0x20 or 0x7F <= cp <= 0x9F:
            if ch in ("\t", "\n", "\r"):
                out.append(" ")
            continue
        out.append(ch)
    cleaned = " ".join("".join(out).split())
    if len(cleaned) > max_len:
        cleaned = cleaned[:max_len].rstrip()
    return cleaned
