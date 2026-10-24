"""The one agent-local tool: ``reply_without_data``."""

from __future__ import annotations

from ..common.tool_specs import ReplyWithoutDataArgs

REASON_FALLBACKS = {
    "greeting": (
        "Привет! Я помогаю с регулярными платежами: что просрочено, что скоро платить, "
        "сколько платим по валютам и кто за что отвечает."
    ),
    "out_of_scope": "Я отвечаю только на вопросы про платежи этой команды.",
    "needs_clarification": "Уточните, пожалуйста, что именно вас интересует по платежам.",
}


def validate_reply(arguments: dict[str, object]) -> ReplyWithoutDataArgs:
    return ReplyWithoutDataArgs.model_validate(arguments)
