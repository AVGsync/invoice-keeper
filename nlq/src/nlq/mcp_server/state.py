"""Process-wide state for the MCP server, set once at startup.

Tools are module-level functions, so they reach the database, settings and the
actor public key through this holder rather than globals scattered per module.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import date

from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PublicKey

from ..config import Settings
from .db import Database

# Payment-schema health, re-checked periodically by selfcheck (section 5.9 p.6-7).
SCHEMA_OK = "ok"
SCHEMA_MISSING = "missing"
SCHEMA_DRIFT = "drift"
SCHEMA_BROKEN = "broken"  # a rendered tool template failed to prepare


@dataclass
class SchemaStatus:
    value: str = SCHEMA_MISSING
    reason: str | None = "payments_schema_missing"
    # Whether B's optional recurring_payments.category column is present.
    category_available: bool = False

    @property
    def payments_available(self) -> bool:
        return self.value == SCHEMA_OK


@dataclass
class AppState:
    settings: Settings
    db: Database
    public_key: Ed25519PublicKey
    schema: SchemaStatus = field(default_factory=SchemaStatus)

    def today(self) -> date:
        from ..common.dates import today

        return today(self.settings.app_timezone, self.settings.today_override)


_state: AppState | None = None


def set_state(state: AppState) -> None:
    global _state
    _state = state


def get_state() -> AppState:
    if _state is None:
        raise RuntimeError("MCP app state is not initialised")
    return _state
