from __future__ import annotations

import base64
import time
from uuid import uuid4

import jwt
import pytest
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey

from nlq.common.actor import AUDIENCE, ISSUER, load_public_key, verify_actor
from nlq.common.errors import InvalidActor
from nlq.devtoken import sign_actor

SUB = str(uuid4())


def _keypair() -> tuple[Ed25519PrivateKey, object]:
    priv = Ed25519PrivateKey.generate()
    pub_raw = priv.public_key().public_bytes(
        serialization.Encoding.Raw, serialization.PublicFormat.Raw
    )
    pub = load_public_key(base64.b64encode(pub_raw).decode("ascii"))
    return priv, pub


def test_valid_token() -> None:
    priv, pub = _keypair()
    token = sign_actor(priv, sub=SUB, role="admin", ch="web", rid="r1")
    actor = verify_actor(token, pub)  # type: ignore[arg-type]
    assert str(actor.sub) == SUB
    assert actor.role == "admin"
    assert actor.ch == "web"
    assert actor.rid == "r1"


def test_expired_rejected() -> None:
    priv, pub = _keypair()
    token = sign_actor(priv, sub=SUB, ttl_seconds=60, now=int(time.time()) - 120)
    with pytest.raises(InvalidActor):
        verify_actor(token, pub)  # type: ignore[arg-type]


def test_wrong_audience_rejected() -> None:
    priv, pub = _keypair()
    now = int(time.time())
    token = jwt.encode(
        {"iss": ISSUER, "aud": "app", "sub": SUB, "role": "user", "ch": "web",
         "iat": now, "nbf": now, "exp": now + 60},
        priv, algorithm="EdDSA",
    )
    with pytest.raises(InvalidActor):
        verify_actor(token, pub)  # type: ignore[arg-type]


def test_wrong_issuer_rejected() -> None:
    priv, pub = _keypair()
    now = int(time.time())
    token = jwt.encode(
        {"iss": "someone-else", "aud": AUDIENCE, "sub": SUB, "role": "user", "ch": "web",
         "iat": now, "nbf": now, "exp": now + 60},
        priv, algorithm="EdDSA",
    )
    with pytest.raises(InvalidActor):
        verify_actor(token, pub)  # type: ignore[arg-type]


def test_hs256_forgery_rejected() -> None:
    _, pub = _keypair()
    now = int(time.time())
    token = jwt.encode(
        {"iss": ISSUER, "aud": AUDIENCE, "sub": SUB, "role": "admin", "ch": "web",
         "iat": now, "nbf": now, "exp": now + 60},
        "a" * 32, algorithm="HS256",
    )
    with pytest.raises(InvalidActor):
        verify_actor(token, pub)  # type: ignore[arg-type]


def test_alg_none_rejected() -> None:
    _, pub = _keypair()
    now = int(time.time())
    token = jwt.encode(
        {"iss": ISSUER, "aud": AUDIENCE, "sub": SUB, "role": "admin", "ch": "web",
         "iat": now, "nbf": now, "exp": now + 60},
        key="", algorithm="none",
    )
    with pytest.raises(InvalidActor):
        verify_actor(token, pub)  # type: ignore[arg-type]


def test_wrong_key_rejected() -> None:
    priv_a, _ = _keypair()
    _, pub_b = _keypair()
    token = sign_actor(priv_a, sub=SUB, role="admin", ch="web")
    with pytest.raises(InvalidActor):
        verify_actor(token, pub_b)  # type: ignore[arg-type]


def test_unknown_role_rejected() -> None:
    priv, pub = _keypair()
    now = int(time.time())
    token = jwt.encode(
        {"iss": ISSUER, "aud": AUDIENCE, "sub": SUB, "role": "root", "ch": "web",
         "iat": now, "nbf": now, "exp": now + 60},
        priv, algorithm="EdDSA",
    )
    with pytest.raises(InvalidActor):
        verify_actor(token, pub)  # type: ignore[arg-type]
