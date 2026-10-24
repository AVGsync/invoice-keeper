"""Entry point: ``python -m nlq.agent`` → uvicorn on :8090 (§3.1)."""

from __future__ import annotations

import logging

import uvicorn

from ..config import get_settings
from .app import create_app


def main() -> None:
    settings = get_settings()
    logging.basicConfig(
        level=getattr(logging, settings.log_level.upper(), logging.INFO),
        format="%(asctime)s %(levelname)s %(name)s %(message)s",
    )
    app = create_app()
    uvicorn.run(app, host=settings.agent_host, port=settings.agent_port, log_level=settings.log_level)


if __name__ == "__main__":
    main()
