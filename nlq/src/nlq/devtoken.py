"""Mint a short-lived actor-JWT for LOCAL testing (MCP Inspector, curl)."""

from __future__ import annotations

import argparse
import base64
import sys
import time
import uuid

import jwt
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey

from .common.actor import AUDIENCE, ISSUER
from .config import get_settings


def load_private_key(seed_b64: str) -> Ed25519PrivateKey:
    raw = base64.b64decode(seed_b64, validate=True)
    if len(raw) != 32:
        raise ValueError("private key seed must be 32 bytes (base64)")
    return Ed25519PrivateKey.from_private_bytes(raw)


def sign_actor(
    private_key: Ed25519PrivateKey,
    *,
    sub: str,
    role: str = "user",
    ch: str = "web",
    rid: str | None = None,
    ttl_seconds: int = 60,
    now: int | None = None,
) -> str:
    issued = now if now is not None else int(time.time())
    claims: dict[str, object] = {
        "iss": ISSUER,
        "aud": AUDIENCE,
        "sub": sub,
        "role": role,
        "ch": ch,
        "iat": issued,
        "nbf": issued,
        "exp": issued + ttl_seconds,
    }
    if rid is not None:
        claims["rid"] = rid
    return jwt.encode(claims, private_key, algorithm="EdDSA")  # type: ignore[arg-type]


def main() -> None:
    settings = get_settings()
    if settings.is_production:
        print("devtoken is refused when APP_ENV=production", file=sys.stderr)
        sys.exit(2)

    parser = argparse.ArgumentParser(prog="nlq.devtoken")
    parser.add_argument("--role", choices=["admin", "user"], default="user")
    parser.add_argument("--ch", choices=["web", "miniapp", "bot"], default="web")
    parser.add_argument("--sub", default=str(uuid.uuid4()), help="user id (UUID)")
    parser.add_argument("--ttl", type=int, default=60, help="seconds until expiry")
    parser.add_argument(
        "--seed",
        default=None,
        help="base64 Ed25519 seed; default from NLQ_ACTOR_PRIVATE_KEY",
    )
    args = parser.parse_args()

    import os

    seed = args.seed or os.environ.get("NLQ_ACTOR_PRIVATE_KEY", "")
    if not seed:
        print("provide --seed or set NLQ_ACTOR_PRIVATE_KEY", file=sys.stderr)
        sys.exit(2)

    token = sign_actor(
        load_private_key(seed),
        sub=args.sub,
        role=args.role,
        ch=args.ch,
        ttl_seconds=args.ttl,
    )
    print(token)


if __name__ == "__main__":
    main()
