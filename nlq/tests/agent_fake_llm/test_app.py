"""HTTP contract of the agent app: /v1/ask, /v1/ask/stream, /healthz."""

from __future__ import annotations

import base64
import json

import pytest
from _agent_fakes import FakeMcpSession, occurrences_envelope, overdue_rows
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from fastapi.testclient import TestClient

from nlq.agent import loop as loop_mod
from nlq.agent.app import create_app
from nlq.agent.fake_llm import FakeLLM
from nlq.agent.service import AgentService
from nlq.config import Settings
from nlq.devtoken import sign_actor


def _keypair() -> tuple[Ed25519PrivateKey, str]:
    priv = Ed25519PrivateKey.generate()
    pub = priv.public_key().public_bytes(
        serialization.Encoding.Raw, serialization.PublicFormat.Raw
    )
    return priv, base64.b64encode(pub).decode("ascii")


def _service(pub_b64: str, script: list[dict]) -> AgentService:
    settings = Settings(
        APP_ENV="local",
        NLQ_LLM="fake",
        NLQ_TODAY="2026-10-24",
        NLQ_ACTOR_PUBLIC_KEY=pub_b64,
        NLQ_MCP_URL="http://mcp/mcp",
    )
    svc = AgentService(settings)
    svc.llm = FakeLLM(script)
    return svc


def _token(priv: Ed25519PrivateKey, role: str = "user", ch: str = "web") -> str:
    return sign_actor(priv, sub="11111111-1111-1111-1111-111111111111", role=role, ch=ch, rid="rid-1")


def test_ask_happy_path(monkeypatch: pytest.MonkeyPatch) -> None:
    priv, pub = _keypair()
    script = [
        {"tool_calls": [{"name": "find_occurrences", "arguments": {"status": "overdue"}}]},
        {"content": "Просрочено 2 платежа."},
    ]
    svc = _service(pub, script)
    monkeypatch.setattr(loop_mod, "McpSession", lambda u, t: FakeMcpSession(lambda n, a: occurrences_envelope(overdue_rows())))
    app = create_app(svc)
    with TestClient(app) as client:
        resp = client.post(
            "/v1/ask",
            headers={"Authorization": f"Bearer {_token(priv)}"},
            json={"question": "что просрочено?", "channel": "web"},
        )
    assert resp.status_code == 200
    body = resp.json()
    assert body["status"] == "ok"
    assert body["mode"] == "fake"
    assert body["answer_id"] == "rid-1"
    assert body["blocks"][0]["type"] == "occurrence_list"


def test_ask_missing_token() -> None:
    _, pub = _keypair()
    app = create_app(_service(pub, []))
    with TestClient(app) as client:
        resp = client.post("/v1/ask", json={"question": "q", "channel": "web"})
    assert resp.status_code == 401
    assert resp.json()["error"]["code"] == "bad_actor"


def test_ask_empty_question() -> None:
    priv, pub = _keypair()
    app = create_app(_service(pub, []))
    with TestClient(app) as client:
        resp = client.post(
            "/v1/ask",
            headers={"Authorization": f"Bearer {_token(priv)}"},
            json={"question": "   ", "channel": "web"},
        )
    assert resp.status_code == 400
    assert resp.json()["error"]["code"] == "bad_question"


def test_ask_channel_mismatch() -> None:
    priv, pub = _keypair()
    app = create_app(_service(pub, []))
    with TestClient(app) as client:
        resp = client.post(
            "/v1/ask",
            headers={"Authorization": f"Bearer {_token(priv, ch='web')}"},
            json={"question": "q", "channel": "miniapp"},
        )
    assert resp.status_code == 400


def test_healthz_fake_mode() -> None:
    _, pub = _keypair()
    app = create_app(_service(pub, []))
    with TestClient(app) as client:
        resp = client.get("/healthz")
    assert resp.status_code == 200
    assert resp.json()["mode"] == "fake"


def test_stream_emits_sse_frames(monkeypatch: pytest.MonkeyPatch) -> None:
    priv, pub = _keypair()
    script = [
        {"tool_calls": [{"name": "find_occurrences", "arguments": {"status": "overdue"}}]},
        {"content": "Просрочено 2 платежа."},
    ]
    svc = _service(pub, script)
    monkeypatch.setattr(loop_mod, "McpSession", lambda u, t: FakeMcpSession(lambda n, a: occurrences_envelope(overdue_rows())))
    app = create_app(svc)
    with TestClient(app) as client, client.stream(
        "POST",
        "/v1/ask/stream",
        headers={"Authorization": f"Bearer {_token(priv)}"},
        json={"question": "что просрочено?", "channel": "web"},
    ) as resp:
        assert resp.status_code == 200
        raw = "".join(resp.iter_text())
    assert "event: step" in raw
    assert "event: block" in raw
    assert "event: done" in raw
    # The final done frame carries the full envelope.
    done = raw.split("event: done")[-1]
    payload = json.loads(done.split("data: ", 1)[1].strip())
    assert payload["status"] == "ok"
