# Один образ для nlq.mcp_server, nlq.agent и nlq.dbinit; команда задаётся в compose.
FROM python:3.14-slim AS base

ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    UV_PROJECT_ENVIRONMENT=/opt/venv \
    UV_COMPILE_BYTECODE=1 \
    UV_LINK_MODE=copy

COPY --from=ghcr.io/astral-sh/uv:0.13.1 /uv /usr/local/bin/uv

WORKDIR /app

COPY pyproject.toml uv.lock ./
RUN uv sync --frozen --no-dev --no-install-project

COPY . .
RUN uv sync --frozen --no-dev

ENV PATH="/opt/venv/bin:$PATH"

# Без root; запись только в tmpfs /tmp.
RUN useradd --system --uid 10001 --no-create-home nlq
USER nlq

ENTRYPOINT ["python", "-m"]
CMD ["nlq.mcp_server"]
