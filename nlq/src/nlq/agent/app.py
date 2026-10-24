"""FastAPI app for nlq-agent: POST /v1/ask, POST /v1/ask/stream (SSE), /healthz.

The agent is internal: it is reached only by the Go backend, which has already
authenticated the user and signed a 60-second actor-JWT. Every request is
verified again here (defence in depth, §8.1) before the loop runs.
"""

from __future__ import annotations

import asyncio
import contextlib
import json
import logging
from collections.abc import AsyncIterator
from contextlib import asynccontextmanager
from typing import Any

from fastapi import FastAPI, Header, Request
from fastapi.responses import JSONResponse, StreamingResponse

from ..common.actor import Actor, verify_actor
from ..common.errors import BadArgs, InvalidActor
from . import loop
from .openrouter import hash_user
from .service import AgentService

log = logging.getLogger("nlq.agent.app")

_CHANNELS = frozenset({"web", "miniapp", "bot"})


def _bearer(authorization: str | None) -> str | None:
    if not authorization:
        return None
    if authorization.lower().startswith("bearer "):
        return authorization[7:].strip()
    return None


def create_app(service: AgentService | None = None) -> FastAPI:
    svc = service or AgentService()

    @asynccontextmanager
    async def lifespan(_app: FastAPI) -> AsyncIterator[None]:
        await svc.startup()
        try:
            yield
        finally:
            await svc.shutdown()

    app = FastAPI(title="nlq-agent", lifespan=lifespan)
    app.state.service = svc

    def _authenticate(authorization: str | None) -> Actor | JSONResponse:
        token = _bearer(authorization)
        if token is None:
            return JSONResponse({"error": {"code": "bad_actor"}}, status_code=401)
        try:
            return verify_actor(token, svc.public_key)
        except InvalidActor:
            return JSONResponse({"error": {"code": "bad_actor"}}, status_code=401)

    async def _prepare(
        request: Request, authorization: str | None
    ) -> tuple[Actor, str, str, str] | JSONResponse:
        """Verify actor + validate body. Returns (actor, question, token, rid)."""
        actor_or_err = _authenticate(authorization)
        if isinstance(actor_or_err, JSONResponse):
            return actor_or_err
        actor = actor_or_err
        token = _bearer(authorization) or ""
        try:
            body = await request.json()
        except (json.JSONDecodeError, ValueError):
            return JSONResponse({"error": {"code": "bad_question"}}, status_code=400)
        if not isinstance(body, dict):
            return JSONResponse({"error": {"code": "bad_question"}}, status_code=400)
        try:
            question = svc.validate_question(body.get("question"))
        except BadArgs:
            return JSONResponse({"error": {"code": "bad_question"}}, status_code=400)
        channel = body.get("channel", "web")
        if channel not in _CHANNELS or channel != actor.ch:
            return JSONResponse({"error": {"code": "bad_question"}}, status_code=400)
        rid = actor.rid or request.headers.get("x-request-id") or ""
        return actor, question, token, rid

    @app.post("/v1/ask")
    async def ask(request: Request, authorization: str | None = Header(default=None)) -> Any:
        prepared = await _prepare(request, authorization)
        if isinstance(prepared, JSONResponse):
            return prepared
        actor, question, token, rid = prepared
        envelope = await svc_answer(svc, actor, question, token, rid)
        return JSONResponse(envelope)

    @app.post("/v1/ask/stream")
    async def ask_stream(
        request: Request, authorization: str | None = Header(default=None)
    ) -> Any:
        prepared = await _prepare(request, authorization)
        if isinstance(prepared, JSONResponse):
            return prepared
        actor, question, token, rid = prepared
        return StreamingResponse(
            _sse(svc, actor, question, token, rid),
            media_type="text/event-stream",
            headers={"Cache-Control": "no-cache", "X-Accel-Buffering": "no"},
        )

    @app.get("/healthz")
    async def healthz() -> Any:
        return JSONResponse(svc.health())

    return app


async def svc_answer(
    svc: AgentService,
    actor: Actor,
    question: str,
    token: str,
    rid: str,
    on_step: loop.StepHook | None = None,
) -> dict[str, Any]:
    scope = svc.effective_scope(actor)
    return await loop.answer(
        question=question,
        actor=actor,
        token=token,
        today=svc.today(),
        scope=scope,
        llm_mode=svc.runtime_mode(),
        llm=svc.llm,
        cfg=svc.make_cfg(),
        user=hash_user(str(actor.sub)),
        answer_id=rid,
        on_step=on_step,
    )


def _sse_event(event: str, data: dict[str, Any]) -> str:
    return f"event: {event}\ndata: {json.dumps(data, ensure_ascii=False)}\n\n"


async def _sse(
    svc: AgentService, actor: Actor, question: str, token: str, rid: str
) -> AsyncIterator[str]:
    """Stream step events as tools run, then the blocks, text and final envelope.

    Text is emitted only after grounding (§9.4), so it arrives in the ``done``
    frame together with the vetted blocks; ``step`` frames stream live.
    """
    queue: asyncio.Queue[tuple[str, dict[str, Any]]] = asyncio.Queue()

    async def on_step(event: str, data: dict[str, Any]) -> None:
        await queue.put((event, data))

    task = asyncio.create_task(svc_answer(svc, actor, question, token, rid, on_step))
    drain: asyncio.Future[tuple[str, dict[str, Any]]] | None = None
    try:
        while True:
            drain = asyncio.ensure_future(queue.get())
            done, _ = await asyncio.wait({drain, task}, return_when=asyncio.FIRST_COMPLETED)
            if drain in done:
                event, data = drain.result()
                yield _sse_event(event, data)
                continue
            drain.cancel()
            drain = None
            break
        while not queue.empty():
            event, data = queue.get_nowait()
            yield _sse_event(event, data)
        envelope = task.result()
        for block in envelope.get("blocks", []):
            yield _sse_event("block", block)
        yield _sse_event("text", {"delta": envelope.get("text", "")})
        yield _sse_event("done", envelope)
    except Exception as exc:  # surface a terminal error frame
        log.exception("stream failed")
        yield _sse_event("error", {"code": "llm_unavailable", "detail": type(exc).__name__})
    finally:
        # On a client disconnect (GeneratorExit/CancelledError) or any error, stop
        # the background work so it does not keep spending LLM/MCP budget, and do
        # not leak the pending queue.get future.
        if drain is not None and not drain.done():
            drain.cancel()
        if not task.done():
            task.cancel()
        with contextlib.suppress(BaseException):
            await task
