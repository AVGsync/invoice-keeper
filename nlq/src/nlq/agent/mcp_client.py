"""MCP client used by the agent to reach nlq-mcp over Streamable HTTP (§7.5).

A fresh client is opened per request and the user's signed actor-JWT is
forwarded on every call, so a MCP restart never wedges the agent and identity
is re-verified server-side on each request. Tools return a JSON envelope string;
here it is parsed back into a dict for the loop and renderer.
"""

from __future__ import annotations

import json
import logging
from types import TracebackType
from typing import Any

from fastmcp import Client
from fastmcp.client.transports import StreamableHttpTransport

log = logging.getLogger("nlq.agent.mcp")


class McpError(Exception):
    """A transport-level failure reaching the MCP server (not a tool error)."""


class McpSession:
    """Async-context wrapper around a connected fastmcp client."""

    def __init__(self, url: str, token: str) -> None:
        transport = StreamableHttpTransport(url, headers={"Authorization": f"Bearer {token}"})
        # init_timeout bounds the MCP handshake so a hung server cannot wedge the
        # request (fastmcp defaults it to None = forever); timeout bounds each call
        # on top of the executor's own asyncio.wait_for (section 7.5).
        self._client = Client(transport, timeout=10.0, init_timeout=2.0)

    async def __aenter__(self) -> McpSession:
        try:
            await self._client.__aenter__()
        except Exception as exc:  # normalise transport failures
            raise McpError(f"cannot reach MCP: {type(exc).__name__}") from exc
        return self

    async def __aexit__(
        self,
        exc_type: type[BaseException] | None,
        exc: BaseException | None,
        tb: TracebackType | None,
    ) -> None:
        await self._client.__aexit__(exc_type, exc, tb)

    async def call(self, name: str, arguments: dict[str, Any]) -> dict[str, Any]:
        """Call a tool and return the parsed envelope dict.

        A transport failure raises McpError; a tool-level error still arrives as
        a well-formed ``{"ok": false, ...}`` envelope.
        """
        try:
            result = await self._client.call_tool(name, arguments)
        except Exception as exc:  # normalise transport failures
            raise McpError(f"MCP call failed: {type(exc).__name__}") from exc
        return _extract_envelope(result, name)


def _extract_envelope(result: Any, name: str) -> dict[str, Any]:
    payload = _result_text(result)
    if payload is None:
        raise McpError("MCP returned no text content")
    try:
        data = json.loads(payload)
    except json.JSONDecodeError as exc:
        raise McpError("MCP returned invalid JSON") from exc
    if not isinstance(data, dict):
        raise McpError("MCP envelope is not an object")
    data.setdefault("tool", name)
    return data


def _result_text(result: Any) -> str | None:
    """Pull the JSON string out of a CallToolResult across fastmcp shapes."""
    data = getattr(result, "data", None)
    if isinstance(data, str):
        return data
    content = getattr(result, "content", None)
    if isinstance(content, list):
        for block in content:
            text = getattr(block, "text", None)
            if isinstance(text, str):
                return text
    if isinstance(result, str):
        return result
    return None
