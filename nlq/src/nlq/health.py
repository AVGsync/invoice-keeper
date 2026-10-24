"""Docker healthcheck: ``python -m nlq.health agent|mcp``."""

from __future__ import annotations

import sys

import httpx

from .config import get_settings

_PORTS = {"agent": 8090, "mcp": 8900}


def main() -> None:
    if len(sys.argv) != 2 or sys.argv[1] not in _PORTS:
        print("usage: python -m nlq.health agent|mcp", file=sys.stderr)
        sys.exit(2)
    which = sys.argv[1]
    settings = get_settings()
    port = settings.mcp_port if which == "mcp" else _PORTS[which]
    url = f"http://127.0.0.1:{port}/healthz"
    try:
        resp = httpx.get(url, timeout=3.0)
    except httpx.HTTPError:
        sys.exit(1)
    sys.exit(0 if resp.status_code == 200 else 1)


if __name__ == "__main__":
    main()
