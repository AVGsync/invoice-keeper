"""Long-lived agent state and per-request helpers."""

from __future__ import annotations

import asyncio
import contextlib
import logging
import unicodedata
from datetime import date

import httpx
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PublicKey

from ..common.actor import Actor, load_public_key
from ..common.dates import today as compute_today
from ..common.errors import BadArgs
from ..common.tool_specs import LLM_TOOL_SCHEMAS
from ..config import Settings, get_settings
from . import fake_llm, selfcheck
from .loop import LLMClient, LoopConfig
from .openrouter import OpenRouterClient

log = logging.getLogger("nlq.agent.service")

MAX_QUESTION_LEN = 500
MCP_PROBE_INTERVAL_SECONDS = 15.0


class AgentService:
    def __init__(self, settings: Settings | None = None) -> None:
        self.settings = settings or get_settings()
        self.public_key: Ed25519PublicKey = load_public_key(self.settings.nlq_actor_public_key)
        self.llm: LLMClient | None = self._build_llm()
        self.model_health: dict[str, str] = {}
        self.mcp_status: str = "unknown"
        self._mcp_task: asyncio.Task[None] | None = None

    def _build_llm(self) -> LLMClient | None:
        mode = self.settings.nlq_llm
        if mode == "rules":
            return None
        if mode == "fake":
            script = (
                fake_llm.load_script(self.settings.nlq_fake_script)
                if self.settings.nlq_fake_script
                else []
            )
            return fake_llm.FakeLLM(script)
        return OpenRouterClient(self.settings)

    async def startup(self) -> None:
        if isinstance(self.llm, OpenRouterClient) and self.settings.openrouter_api_key:
            self.model_health = await selfcheck.check_models(self.settings, self.llm)
            log.info("model health: %s", self.model_health)
        self._mcp_task = asyncio.create_task(self._mcp_probe_loop())

    async def shutdown(self) -> None:
        if self._mcp_task is not None:
            self._mcp_task.cancel()
            with contextlib.suppress(asyncio.CancelledError):
                await self._mcp_task
        if self.llm is not None:
            await self.llm.aclose()

    def _mcp_healthz_url(self) -> str:
        """Derive the MCP /healthz URL from NLQ_MCP_URL (…/mcp → …/healthz)."""
        base = self.settings.nlq_mcp_url.rstrip("/")
        if base.endswith("/mcp"):
            base = base[: -len("/mcp")]
        return f"{base}/healthz"

    async def _probe_mcp_once(self) -> str:
        url = self._mcp_healthz_url()
        try:
            async with httpx.AsyncClient(timeout=httpx.Timeout(2.0)) as client:
                resp = await client.get(url)
            if resp.status_code != 200:
                return "down"
            data = resp.json()
        except Exception:
            return "down"
        status = data.get("status")
        schema = data.get("payments_schema")
        if status == "ok" and schema == "ok":
            return "ok"
        return "degraded"

    async def _mcp_probe_loop(self) -> None:
        while True:
            self.mcp_status = await self._probe_mcp_once()
            await asyncio.sleep(MCP_PROBE_INTERVAL_SECONDS)


    def today(self) -> date:
        return compute_today(self.settings.app_timezone, self.settings.today_override)

    def effective_scope(self, actor: Actor) -> str:
        if actor.role == "admin":
            return "team"
        return "own" if self.settings.nlq_user_scope == "own" else "team"

    def make_cfg(self) -> LoopConfig:
        s = self.settings
        return LoopConfig(
            tools=LLM_TOOL_SCHEMAS,
            max_llm_calls=s.nlq_max_llm_calls,
            max_tool_calls_per_step=s.nlq_max_tool_calls_per_step,
            step_timeout=s.nlq_step_timeout,
            tool_timeout=s.nlq_tool_timeout,
            total_timeout=s.nlq_total_timeout,
            mcp_url=s.nlq_mcp_url,
            rules_fallback=s.nlq_rules_fallback,
            tz=s.app_timezone,
        )

    def validate_question(self, question: object) -> str:
        if not isinstance(question, str):
            raise BadArgs("question must be a string")
        q = question.strip()
        runes = len(q)
        if runes == 0:
            raise BadArgs("question must not be empty")
        if runes > MAX_QUESTION_LEN:
            raise BadArgs(f"question must be <= {MAX_QUESTION_LEN} characters")
        for ch in q:
            if ch in ("\n", "\t"):
                continue
            if unicodedata.category(ch).startswith("C"):
                raise BadArgs("question contains control characters")
        return q

    def runtime_mode(self) -> str:
        return self.settings.nlq_llm

    def health(self) -> dict[str, object]:
        mode = self.settings.nlq_llm
        degraded_models = {
            m: st for m, st in self.model_health.items() if st != "ok"
        }
        status = "ok"
        reason: str | None = None
        if mode == "openrouter":
            if not self.settings.openrouter_api_key:
                status, reason = "degraded", "no_openrouter_key"
            elif degraded_models:
                status, reason = "degraded", "model_missing_tools"
            elif self.mcp_status != "ok":
                status, reason = "degraded", f"mcp_{self.mcp_status}"
        return {
            "status": status,
            "mode": mode,
            "models": self.model_health,
            "mcp": self.mcp_status,
            "reason": reason,
        }
