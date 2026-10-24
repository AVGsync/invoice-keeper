"""Authentication for the MCP HTTP app.

An ASGI middleware rejects any request without a valid actor-JWT with 401
(except ``/healthz``) and caps the request body at 64 KB (§5.8). Inside a tool,
``current_actor()`` re-reads and re-verifies the ``Authorization`` header, so
identity never depends on how the library propagates context between tasks
(§5.1 p.1).
"""

from __future__ import annotations

import json
import logging
from typing import Final

from fastmcp.server.dependencies import get_http_request
from starlette.types import ASGIApp, Message, Receive, Scope, Send

from ..common.actor import Actor, verify_actor
from ..common.errors import InvalidActor
from .state import get_state

log = logging.getLogger("nlq.mcp.auth")

MAX_BODY_BYTES: Final = 64 * 1024
_HEALTH_PATHS: Final = frozenset({"/healthz"})


def _bearer(headers: list[tuple[bytes, bytes]]) -> str | None:
    for key, value in headers:
        if key.lower() == b"authorization":
            text = value.decode("latin-1")
            if text.lower().startswith("bearer "):
                return text[7:].strip()
            return None
    return None


async def _send_json(send: Send, status: int, code: str) -> None:
    body = json.dumps({"error": {"code": code}}).encode("utf-8")
    await send(
        {
            "type": "http.response.start",
            "status": status,
            "headers": [
                (b"content-type", b"application/json"),
                (b"content-length", str(len(body)).encode("ascii")),
            ],
        }
    )
    await send({"type": "http.response.body", "body": body})


class AuthMiddleware:
    """Pure-ASGI guard: body-size cap + mandatory actor-JWT."""

    def __init__(self, app: ASGIApp) -> None:
        self.app = app

    async def __call__(self, scope: Scope, receive: Receive, send: Send) -> None:
        if scope["type"] != "http":
            await self.app(scope, receive, send)
            return

        path = scope.get("path", "")
        if path in _HEALTH_PATHS:
            await self.app(scope, receive, send)
            return

        headers: list[tuple[bytes, bytes]] = scope.get("headers", [])
        for key, value in headers:
            if key.lower() == b"content-length":
                try:
                    if int(value) > MAX_BODY_BYTES:
                        await _send_json(send, 413, "payload_too_large")
                        return
                except ValueError:
                    pass

        token = _bearer(headers)
        if token is None:
            await _send_json(send, 401, "missing_actor")
            return
        try:
            verify_actor(token, get_state().public_key)
        except InvalidActor:
            await _send_json(send, 401, "bad_actor")
            return

        await self.app(scope, _capped_receive(receive), send)


def _capped_receive(receive: Receive) -> Receive:
    """Reject a body that exceeds the cap even when Content-Length lies."""
    seen = 0

    async def wrapped() -> Message:
        nonlocal seen
        message = await receive()
        if message["type"] == "http.request":
            seen += len(message.get("body", b""))
            if seen > MAX_BODY_BYTES:
                raise _TooLarge
        return message

    return wrapped


class _TooLargeError(Exception):
    pass


_TooLarge = _TooLargeError()


def current_actor() -> Actor:
    """Re-verify the actor from the live HTTP request inside a tool."""
    request = get_http_request()
    token = _bearer(list(request.headers.raw))
    if token is None:
        raise InvalidActor("missing actor token")
    return verify_actor(token, get_state().public_key)
