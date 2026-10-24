"""OpenRouter client parsing, retries and error mapping (§7.1-7.3)."""

from __future__ import annotations

import httpx
import pytest
import respx

from nlq.agent.openrouter import OpenRouterClient, OpenRouterError, hash_user
from nlq.config import Settings

pytestmark = pytest.mark.asyncio

BASE = "https://openrouter.test/api/v1"


def _settings() -> Settings:
    return Settings(
        OPENROUTER_API_KEY="sk-test",
        OPENROUTER_BASE_URL=BASE,
        NLQ_MODELS="primary/model,backup/model",
        NLQ_ACTOR_PUBLIC_KEY="",
        NLQ_MAX_TOKENS=700,
    )


def _ok_body(content: str | None = "ответ", tool: bool = False) -> dict:
    message: dict = {"role": "assistant", "content": content}
    if tool:
        message["tool_calls"] = [
            {"id": "c1", "type": "function",
             "function": {"name": "find_occurrences", "arguments": '{"status":"overdue"}'}}
        ]
        message["content"] = None
    return {
        "model": "primary/model",
        "choices": [{"message": message, "finish_reason": "tool_calls" if tool else "stop"}],
        "usage": {"prompt_tokens": 100, "completion_tokens": 20, "cost": 0.0012},
    }


async def test_parses_tool_calls_and_usage() -> None:
    async with respx.mock:
        respx.post(f"{BASE}/chat/completions").mock(
            return_value=httpx.Response(200, json=_ok_body(tool=True))
        )
        client = OpenRouterClient(_settings())
        resp = await client.chat([], [], "required", timeout=5, model="primary/model")
        assert resp.tool_calls[0].name == "find_occurrences"
        assert resp.prompt_tokens == 100
        assert resp.cost_usd == pytest.approx(0.0012)
        await client.aclose()


async def test_transient_then_success_retries_once() -> None:
    async with respx.mock:
        route = respx.post(f"{BASE}/chat/completions")
        route.side_effect = [
            httpx.Response(503),
            httpx.Response(200, json=_ok_body()),
        ]
        client = OpenRouterClient(_settings())
        resp = await client.chat([], [], "auto", timeout=5, model="primary/model")
        assert resp.content == "ответ"
        assert route.call_count == 2
        await client.aclose()


async def test_auth_error_flagged() -> None:
    async with respx.mock:
        respx.post(f"{BASE}/chat/completions").mock(return_value=httpx.Response(401, json={}))
        client = OpenRouterClient(_settings())
        with pytest.raises(OpenRouterError) as exc:
            await client.chat([], [], "auto", timeout=5, model="primary/model")
        assert exc.value.auth is True
        await client.aclose()


async def test_200_with_error_object_raises() -> None:
    async with respx.mock:
        respx.post(f"{BASE}/chat/completions").mock(
            return_value=httpx.Response(200, json={"error": {"message": "bad"}})
        )
        client = OpenRouterClient(_settings())
        with pytest.raises(OpenRouterError):
            await client.chat([], [], "auto", timeout=5, model="primary/model")
        await client.aclose()


async def test_hash_user_is_stable_and_short() -> None:
    h = hash_user("11111111-1111-1111-1111-111111111111")
    assert len(h) == 16
    assert h == hash_user("11111111-1111-1111-1111-111111111111")
