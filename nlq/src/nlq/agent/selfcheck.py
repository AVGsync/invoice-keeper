"""Agent startup / health self-check (§7.2).

Confirms each configured model advertises ``tools`` and ``tool_choice`` on
OpenRouter. A missing capability makes ``/healthz`` degraded with a reason; it
does not crash the agent (rules fallback still answers).
"""

from __future__ import annotations

import logging
from typing import Any

from ..config import Settings
from .openrouter import OpenRouterClient

log = logging.getLogger("nlq.agent.selfcheck")

_REQUIRED = ("tools", "tool_choice")


async def check_models(settings: Settings, client: OpenRouterClient) -> dict[str, str]:
    """Return {model_id: 'ok' | 'missing_tools' | 'unknown'} for NLQ_MODELS."""
    statuses: dict[str, str] = {}
    try:
        catalogue = await client.list_models()
    except Exception as exc:  # network/catalogue failure is non-fatal
        log.warning("could not fetch /models: %s", type(exc).__name__)
        return {m: "unknown" for m in settings.models}

    for model in settings.models:
        info = catalogue.get(model)
        if info is None:
            statuses[model] = "unknown"
            continue
        supported = set(_supported_parameters(info))
        statuses[model] = "ok" if all(p in supported for p in _REQUIRED) else "missing_tools"
    return statuses


def _supported_parameters(info: dict[str, Any]) -> list[str]:
    params = info.get("supported_parameters")
    return params if isinstance(params, list) else []
