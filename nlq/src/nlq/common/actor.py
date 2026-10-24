"""Actor identity carried by a signed EdDSA (Ed25519) JWT."""

from __future__ import annotations

import base64
import binascii
from dataclasses import dataclass
from typing import Final
from uuid import UUID

import jwt
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PublicKey

from .errors import InvalidActor

ISSUER: Final = "invoice-keeper-backend"
AUDIENCE: Final = "nlq"
LEEWAY_SECONDS: Final = 5
_ROLES: Final = frozenset({"admin", "user"})
_CHANNELS: Final = frozenset({"web", "miniapp", "bot"})


@dataclass(frozen=True, slots=True)
class Actor:
    sub: UUID
    role: str
    ch: str
    rid: str | None = None


def load_public_key(public_key_b64: str) -> Ed25519PublicKey:
    """Load an Ed25519 public key from base64 of its 32 raw bytes."""
    try:
        raw = base64.b64decode(public_key_b64, validate=True)
    except (binascii.Error, ValueError) as exc:
        raise InvalidActor("actor public key is not valid base64") from exc
    if len(raw) != 32:
        raise InvalidActor("actor public key must be 32 bytes")
    return Ed25519PublicKey.from_public_bytes(raw)


def verify_actor(token: str, public_key: Ed25519PublicKey) -> Actor:
    """Verify signature, aud, iss, exp and return the Actor, else raise InvalidActor."""
    try:
        claims = jwt.decode(
            token,
            public_key,  # type: ignore[arg-type]
            algorithms=["EdDSA"],
            audience=AUDIENCE,
            issuer=ISSUER,
            leeway=LEEWAY_SECONDS,
            options={"require": ["exp", "iat", "sub", "aud", "iss"]},
        )
    except jwt.InvalidTokenError as exc:
        raise InvalidActor(f"actor token rejected: {type(exc).__name__}") from exc

    role = claims.get("role")
    ch = claims.get("ch")
    if role not in _ROLES or ch not in _CHANNELS:
        raise InvalidActor("actor token has unknown role or channel")
    try:
        sub = UUID(str(claims["sub"]))
    except (ValueError, KeyError) as exc:
        raise InvalidActor("actor token sub is not a UUID") from exc

    rid = claims.get("rid")
    return Actor(sub=sub, role=role, ch=ch, rid=str(rid) if rid is not None else None)
