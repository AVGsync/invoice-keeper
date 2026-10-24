"""OpenRouter chat-completions client (§7.1–7.3).

httpx is used directly (not the OpenAI SDK) so ``provider``, ``models`` and
``usage`` fields pass through transparently. The client performs *one* transient
retry per call; model fallback and the rules fallback are orchestrated by the
loop, which knows the deadline.
"""

from __future__ import annotations

import asyncio
import hashlib
import json
import logging
from typing import Any

import httpx

from ..config import Settings
from .types import LLMResponse, ToolCall

log = logging.getLogger("nlq.agent.openrouter")


class OpenRouterError(Exception):
    """A call that should trigger model/rules fallback.

    ``retryable`` means a single transient retry already happened (or should be
    skipped for auth/credit errors). ``auth`` flags 401/402/403 so the loop logs
    an alert and never retries (§7.3).
    """

    def __init__(self, message: str, *, auth: bool = False) -> None:
        super().__init__(message)
        self.auth = auth


_TRANSIENT_STATUS = frozenset({408, 409, 429, 500, 502, 503, 504})
_AUTH_STATUS = frozenset({401, 402, 403})


def hash_user(user_id: str) -> str:
    """Pseudonymous ``user`` field for OpenRouter: sha256(user_id)[:16] (§8.5)."""
    return hashlib.sha256(user_id.encode("utf-8")).hexdigest()[:16]


class OpenRouterClient:
    def __init__(self, settings: Settings, client: httpx.AsyncClient | None = None) -> None:
        self._settings = settings
        self._models = settings.models
        self._client = client or httpx.AsyncClient(timeout=httpx.Timeout(30.0))
        self._owns_client = client is None

    @property
    def primary(self) -> str:
        return self._models[0] if self._models else ""

    @property
    def backup(self) -> str | None:
        return self._models[1] if len(self._models) > 1 else None

    async def aclose(self) -> None:
        if self._owns_client:
            await self._client.aclose()

    async def chat(
        self,
        messages: list[dict[str, Any]],
        tools: list[dict[str, Any]],
        tool_choice: str,
        *,
        timeout: float,
        model: str,
        fallback_models: list[str] | None = None,
        user: str | None = None,
    ) -> LLMResponse:
        """One logical completion. Raises OpenRouterError to signal fallback."""
        body: dict[str, Any] = {
            "model": model,
            "messages": messages,
            "tools": tools,
            "tool_choice": tool_choice,
            "temperature": 0,
            "max_tokens": self._settings.nlq_max_tokens,
            "provider": {
                "require_parameters": True,
                "allow_fallbacks": True,
                "data_collection": "deny",
            },
            "usage": {"include": True},
        }
        if fallback_models:
            body["models"] = [model, *fallback_models]
        if user:
            body["user"] = user

        headers = {
            "Authorization": f"Bearer {self._settings.openrouter_api_key}",
            "HTTP-Referer": self._settings.nlq_public_url,
            "X-Title": "Invoice Keeper",
            "Content-Type": "application/json",
        }
        url = f"{self._settings.openrouter_base_url.rstrip('/')}/chat/completions"

        data = await self._post_with_retry(url, headers, body, timeout)
        return _parse_response(data)

    async def _post_with_retry(
        self,
        url: str,
        headers: dict[str, str],
        body: dict[str, Any],
        timeout: float,
    ) -> dict[str, Any]:
        # Bound each phase so one slow attempt cannot consume the whole budget:
        # connect/write/pool are capped low, read gets the step budget (section 7.3).
        per_attempt = httpx.Timeout(
            timeout,
            connect=min(2.0, timeout),
            write=min(2.0, timeout),
            pool=min(2.0, timeout),
        )
        for attempt in range(2):
            try:
                resp = await self._client.post(url, headers=headers, json=body, timeout=per_attempt)
            except (httpx.TimeoutException, httpx.TransportError) as exc:
                if attempt == 0:
                    await asyncio.sleep(0.5)
                    continue
                raise OpenRouterError(f"network error: {type(exc).__name__}") from exc

            if resp.status_code in _AUTH_STATUS:
                raise OpenRouterError(
                    f"openrouter_auth_or_credits: HTTP {resp.status_code}", auth=True
                )
            if resp.status_code in _TRANSIENT_STATUS:
                if attempt == 0 and _should_retry(resp):
                    await asyncio.sleep(0.5)
                    continue
                raise OpenRouterError(f"HTTP {resp.status_code}")
            if resp.status_code >= 400:
                # 400 and friends: do not retry here; the loop may try backup.
                log.warning("openrouter rejected request: HTTP %s", resp.status_code)
                raise OpenRouterError(f"HTTP {resp.status_code}")

            try:
                data = resp.json()
            except json.JSONDecodeError as exc:
                raise OpenRouterError("response is not JSON") from exc

            if isinstance(data, dict) and data.get("error"):
                raise OpenRouterError("response carried an error object")
            if not isinstance(data, dict) or not data.get("choices"):
                raise OpenRouterError("response had no choices")
            return data
        raise OpenRouterError("exhausted retries")  # pragma: no cover

    async def list_models(self, timeout: float = 5.0) -> dict[str, dict[str, Any]]:
        """``GET /models`` → {id: model} for the self-check (§7.2)."""
        url = f"{self._settings.openrouter_base_url.rstrip('/')}/models"
        headers = {"Authorization": f"Bearer {self._settings.openrouter_api_key}"}
        resp = await self._client.get(url, headers=headers, timeout=timeout)
        resp.raise_for_status()
        data = resp.json()
        return {m["id"]: m for m in data.get("data", []) if isinstance(m, dict) and m.get("id")}


def _should_retry(resp: httpx.Response) -> bool:
    if resp.status_code == 429:
        retry_after = resp.headers.get("retry-after")
        if retry_after is not None:
            try:
                return float(retry_after) <= 3
            except ValueError:
                return False
    return True


def _parse_response(data: dict[str, Any]) -> LLMResponse:
    choice = data["choices"][0]
    finish_reason = choice.get("finish_reason")
    # A 200 that still failed: a choice-level error or finish_reason=="error"
    # must trigger the backup, not be shown as an empty answer (section 7.3).
    if choice.get("error") or finish_reason == "error":
        raise OpenRouterError("choice-level error in a 200 response")
    message = choice.get("message") or {}
    content = message.get("content")
    tool_calls = _parse_tool_calls(message.get("tool_calls"))
    usage = data.get("usage") or {}
    return LLMResponse(
        content=content if isinstance(content, str) else None,
        tool_calls=tool_calls,
        finish_reason=finish_reason,
        model=data.get("model"),
        raw_message=_clean_history_message(message),
        prompt_tokens=int(usage.get("prompt_tokens") or 0),
        completion_tokens=int(usage.get("completion_tokens") or 0),
        cost_usd=_coerce_cost(usage.get("cost")),
    )


def _parse_tool_calls(raw: Any) -> list[ToolCall]:
    if not isinstance(raw, list):
        return []
    calls: list[ToolCall] = []
    for i, item in enumerate(raw):
        if not isinstance(item, dict):
            continue
        fn = item.get("function") or {}
        name = fn.get("name")
        if not isinstance(name, str):
            continue
        args = fn.get("arguments")
        calls.append(
            ToolCall(
                id=str(item.get("id") or f"call_{i}"),
                name=name,
                arguments=args if isinstance(args, str) else json.dumps(args or {}),
            )
        )
    return calls


def _clean_history_message(message: dict[str, Any]) -> dict[str, Any]:
    """Keep the assistant turn appended to history valid for the next request."""
    out: dict[str, Any] = {"role": message.get("role", "assistant")}
    if message.get("content") is not None:
        out["content"] = message["content"]
    if message.get("tool_calls"):
        out["tool_calls"] = message["tool_calls"]
    # Reasoning models (Gemini/Anthropic thinking) may reject a follow-up turn
    # whose prior assistant message dropped these, so carry them as-is (section 7.1).
    for k in ("reasoning", "reasoning_details"):
        if message.get(k) is not None:
            out[k] = message[k]
    if out.get("content") is None and not out.get("tool_calls"):
        out["content"] = ""
    return out


def _coerce_cost(value: Any) -> float | None:
    if value is None:
        return None
    try:
        return float(value)
    except (TypeError, ValueError):
        return None
