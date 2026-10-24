"""validate_question — общий с Go (backend ValidateQuestion) набор правил (§9.1).

Go-зеркало: backend/internal/service/assistant.go ValidateQuestion и его тест
TestValidateQuestion. Оба отвергают любую руну категории Unicode C, кроме \n/\t,
и считают длину по всем рунам (включая переводы строк).
"""

from __future__ import annotations

import base64

import pytest
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey

from nlq.agent.service import AgentService, BadArgs
from nlq.config import Settings


def _service() -> AgentService:
    priv = Ed25519PrivateKey.generate()
    pub = priv.public_key().public_bytes(
        serialization.Encoding.Raw, serialization.PublicFormat.Raw
    )
    settings = Settings(
        APP_ENV="local",
        NLQ_LLM="fake",
        NLQ_ACTOR_PUBLIC_KEY=base64.b64encode(pub).decode("ascii"),
        NLQ_MCP_URL="http://mcp/mcp",
    )
    return AgentService(settings)


def test_valid_question_passes() -> None:
    svc = _service()
    assert svc.validate_question("  что просрочено?  ") == "что просрочено?"


def test_newline_and_tab_allowed() -> None:
    svc = _service()
    assert svc.validate_question("строка1\nстрока2\tконец")


@pytest.mark.parametrize(
    "bad",
    [
        "что‍просрочено",  # ZWJ (Cf)
        "что﻿просрочено",  # BOM (Cf)
        "что­просрочено",  # soft hyphen (Cf)
        "кто это 👨‍💻",  # ZWJ-эмодзи
        "что\x00просрочено",  # C0 control
    ],
)
def test_control_and_format_chars_rejected(bad: str) -> None:
    svc = _service()
    with pytest.raises(BadArgs):
        svc.validate_question(bad)


def test_newlines_count_toward_limit() -> None:
    svc = _service()
    # Перевод в середине не срежется strip(): 250 + 1 + 250 = 501 руны → отказ.
    with pytest.raises(BadArgs):
        svc.validate_question("я" * 250 + "\n" + "я" * 250)
