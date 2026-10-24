"""Stable error codes shared by the MCP tools and the agent."""

from __future__ import annotations

from typing import Final

BAD_ARGS: Final = "bad_args"
FORBIDDEN: Final = "forbidden"
TIMEOUT: Final = "timeout"
DB_UNAVAILABLE: Final = "db_unavailable"
SOURCE_UNAVAILABLE: Final = "source_unavailable"

ERROR_CODES: Final = frozenset(
    {BAD_ARGS, FORBIDDEN, TIMEOUT, DB_UNAVAILABLE, SOURCE_UNAVAILABLE}
)


class NlqError(Exception):
    """Base class for tool errors mapped to a stable code."""

    code: str = BAD_ARGS

    def __init__(self, message: str) -> None:
        super().__init__(message)
        self.message = message


class BadArgs(NlqError):
    code = BAD_ARGS


class Forbidden(NlqError):
    code = FORBIDDEN


class Timeout(NlqError):
    code = TIMEOUT


class DbUnavailable(NlqError):
    code = DB_UNAVAILABLE


class SourceUnavailable(NlqError):
    code = SOURCE_UNAVAILABLE


class InvalidActor(Exception):
    """Raised when an actor token fails verification."""
