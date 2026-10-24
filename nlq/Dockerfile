# Single image for all three commands: nlq.mcp_server, nlq.agent, nlq.dbinit.
# Pick the command via `command:` in compose (ENTRYPOINT is `python -m`).
FROM python:3.14-slim AS base

ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    UV_PROJECT_ENVIRONMENT=/opt/venv \
    UV_COMPILE_BYTECODE=1 \
    UV_LINK_MODE=copy

COPY --from=ghcr.io/astral-sh/uv:0.13.1 /uv /usr/local/bin/uv

WORKDIR /app

# Install dependencies first (cacheable), then the project.
COPY pyproject.toml uv.lock ./
RUN uv sync --frozen --no-dev --no-install-project

COPY . .
RUN uv sync --frozen --no-dev

ENV PATH="/opt/venv/bin:$PATH"

# Non-root, read-only-friendly (write only to tmpfs /tmp in compose).
RUN useradd --system --uid 10001 --no-create-home nlq
USER nlq

ENTRYPOINT ["python", "-m"]
CMD ["nlq.mcp_server"]
