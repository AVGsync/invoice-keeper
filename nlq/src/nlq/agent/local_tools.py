"""The one agent-local tool: ``reply_without_data`` (§5.7).

It never touches the database. It exists so the first LLM step can be
``tool_choice="required"`` while greetings / off-topic / clarification still
finish without a DB round-trip. Its message goes through the same cleaning and
grounding (digits are not allowed in it).
"""

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
