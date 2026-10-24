"""Entry point: ``python -m nlq.mcp_server`` (serves MCP Streamable HTTP + /healthz)."""

from __future__ import annotations

import logging
import sys

import uvicorn
from starlette.middleware import Middleware

from ..common.actor import load_public_key
from ..config import get_settings
from .auth import AuthMiddleware
from .db import Database
from .server import mcp
from .state import AppState, set_state

log = logging.getLogger("nlq.mcp")


def main() -> None:
    settings = get_settings()
    logging.basicConfig(
        level=settings.log_level.upper(),
        format="%(asctime)s %(levelname)s %(name)s %(message)s",
    )
    if not settings.nlq_db_dsn:
        log.error("NLQ_DB_DSN is required")
        sys.exit(2)
    if not settings.nlq_actor_public_key:
        log.error("NLQ_ACTOR_PUBLIC_KEY is required")
        sys.exit(2)

    public_key = load_public_key(settings.nlq_actor_public_key)
    db = Database(settings.nlq_db_dsn, dev_schema=settings.dev_schema)
    set_state(AppState(settings=settings, db=db, public_key=public_key))

    app = mcp.http_app(
        path="/mcp",
        stateless_http=True,
        transport="streamable-http",
        middleware=[Middleware(AuthMiddleware)],
    )
    uvicorn.run(app, host=settings.mcp_host, port=settings.mcp_port, log_level=settings.log_level)


if __name__ == "__main__":
    main()
