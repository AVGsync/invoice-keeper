"""Single source of truth for tool arguments.

Each tool is defined once as a pydantic model (``extra="forbid"`` → unknown
fields such as ``user_id``/``scope``/``today`` become ``bad_args``) together
with a *flat* JSON schema for the LLM (no ``$ref``/``anyOf``/``default``/
``format`` so it survives ``provider.require_parameters`` across vendors, §5.3).
A unit test asserts the two stay in sync on enums and required fields.
"""

from __future__ import annotations

from typing import Any, Final, Literal
from uuid import UUID

from pydantic import BaseModel, ConfigDict, Field, model_validator

Status = Literal["overdue", "upcoming", "unpaid", "paid", "any"]
Currency = Literal["RUB", "USD", "EUR"]
PeriodArg = Literal[
    "today", "this_week", "next_7_days", "this_month", "next_month", "this_year", "custom"
]

_DATE_PATTERN = r"^\d{4}-\d{2}-\d{2}$"


class _Args(BaseModel):
    model_config = ConfigDict(extra="forbid")


def _validate_assignee(value: str | None) -> None:
    if value is None or value == "me":
        return
    try:
        UUID(value)
    except ValueError as exc:
        raise ValueError("assignee_id must be a UUID or the string 'me'") from exc


class FindOccurrencesArgs(_Args):
    status: Status
    period: PeriodArg | None = None
    date_from: str | None = Field(default=None, pattern=_DATE_PATTERN)
    date_to: str | None = Field(default=None, pattern=_DATE_PATTERN)
    currency: Currency | None = None
    min_amount: float | None = Field(default=None, ge=0)
    max_amount: float | None = Field(default=None, ge=0)
    title_query: str | None = Field(default=None, min_length=1, max_length=64)
    category: str | None = Field(default=None, min_length=1, max_length=64)
    assignee_id: str | None = None
    sort: Literal["due_date", "amount_desc"] | None = None
    limit: int | None = Field(default=None, ge=1, le=20)

    @model_validator(mode="after")
    def _check(self) -> FindOccurrencesArgs:
        _require_currency_for_amounts(self.min_amount, self.max_amount, self.currency)
        _validate_assignee(self.assignee_id)
        return self


class SumOccurrencesArgs(_Args):
    status: Status
    period: PeriodArg | None = None
    date_from: str | None = Field(default=None, pattern=_DATE_PATTERN)
    date_to: str | None = Field(default=None, pattern=_DATE_PATTERN)
    currency: Currency | None = None
    title_query: str | None = Field(default=None, min_length=1, max_length=64)
    category: str | None = Field(default=None, min_length=1, max_length=64)
    assignee_id: str | None = None
    group_by: Literal["currency", "currency_assignee"] = "currency"
    basis: Literal["period_sum", "run_rate"] = "period_sum"

    @model_validator(mode="after")
    def _check(self) -> SumOccurrencesArgs:
        _validate_assignee(self.assignee_id)
        return self


class FindRecurringPaymentsArgs(_Args):
    title_query: str | None = Field(default=None, min_length=1, max_length=64)
    category: str | None = Field(default=None, min_length=1, max_length=64)
    currency: Currency | None = None
    min_amount: float | None = Field(default=None, ge=0)
    max_amount: float | None = Field(default=None, ge=0)
    assignee_id: str | None = None
    include_ended: bool = False
    sort: Literal["title", "amount_desc", "next_due"] | None = None
    limit: int | None = Field(default=None, ge=1, le=20)

    @model_validator(mode="after")
    def _check(self) -> FindRecurringPaymentsArgs:
        _require_currency_for_amounts(self.min_amount, self.max_amount, self.currency)
        _validate_assignee(self.assignee_id)
        return self


class FindPeopleArgs(_Args):
    query: str | None = Field(default=None, min_length=1, max_length=64)
    limit: int | None = Field(default=None, ge=1, le=20)


class ReplyWithoutDataArgs(_Args):
    reason: Literal["greeting", "out_of_scope", "needs_clarification"]
    message: str = Field(min_length=1, max_length=1000)


def _require_currency_for_amounts(
    min_amount: float | None, max_amount: float | None, currency: Currency | None
) -> None:
    if (min_amount is not None or max_amount is not None) and currency is None:
        raise ValueError("min_amount/max_amount require a currency")


# --- Flat JSON schemas for the LLM (OpenAI function-calling format) ---

FIND_OCCURRENCES_SCHEMA: Final[dict[str, Any]] = {
    "type": "function",
    "function": {
        "name": "find_occurrences",
        "description": (
            "Найти периоды оплаты (начисления) регулярных платежей: что просрочено, что "
            "скоро платить, что оплачено. Даты — сроки оплаты. Валюты не пересчитываются. "
            "Возвращает до 20 строк и общее число."
        ),
        "parameters": {
            "type": "object",
            "additionalProperties": False,
            "properties": {
                "status": {
                    "type": "string",
                    "enum": ["overdue", "upcoming", "unpaid", "paid", "any"],
                    "description": (
                        "overdue — срок прошёл и не оплачено; upcoming — срок сегодня или "
                        "позже, не оплачено; unpaid — все неоплаченные; paid — оплаченные; "
                        "any — всё"
                    ),
                },
                "period": {
                    "type": "string",
                    "enum": [
                        "today", "this_week", "next_7_days", "this_month",
                        "next_month", "this_year", "custom",
                    ],
                    "description": (
                        "Готовый интервал по сроку оплаты. custom — тогда обязательны "
                        "date_from и date_to. Не указывай, если интервал не нужен"
                    ),
                },
                "date_from": {
                    "type": "string",
                    "pattern": _DATE_PATTERN,
                    "description": "Только при period=custom, YYYY-MM-DD включительно",
                },
                "date_to": {
                    "type": "string",
                    "pattern": _DATE_PATTERN,
                    "description": "Только при period=custom, YYYY-MM-DD включительно",
                },
                "currency": {"type": "string", "enum": ["RUB", "USD", "EUR"]},
                "min_amount": {
                    "type": "number",
                    "description": "Сумма больше или равна, в указанной валюте",
                },
                "max_amount": {
                    "type": "number",
                    "description": "Сумма меньше или равна, в указанной валюте",
                },
                "title_query": {
                    "type": "string",
                    "description": "Слово из названия платежа, например аренда, хостинг",
                },
                "category": {
                    "type": "string",
                    "description": (
                        "Категория платежа, если она есть в системе. Фильтр мягкий: "
                        "если категорий нет, он игнорируется"
                    ),
                },
                "assignee_id": {
                    "type": "string",
                    "description": "id ответственного из find_people или строка me",
                },
                "sort": {"type": "string", "enum": ["due_date", "amount_desc"]},
                "limit": {"type": "integer", "minimum": 1, "maximum": 20},
            },
            "required": ["status"],
        },
    },
}

SUM_OCCURRENCES_SCHEMA: Final[dict[str, Any]] = {
    "type": "function",
    "function": {
        "name": "sum_occurrences",
        "description": (
            "Посчитать суммы периодов оплаты отдельно по каждой валюте (RUB, USD, EUR). "
            "Общий итог по разным валютам не считается и курсы не применяются. Используй "
            "для вопросов «сколько платим/потратили/должны»."
        ),
        "parameters": {
            "type": "object",
            "additionalProperties": False,
            "properties": {
                "status": {
                    "type": "string",
                    "enum": ["overdue", "upcoming", "unpaid", "paid", "any"],
                },
                "period": {
                    "type": "string",
                    "enum": [
                        "today", "this_week", "next_7_days", "this_month",
                        "next_month", "this_year", "custom",
                    ],
                },
                "date_from": {"type": "string", "pattern": _DATE_PATTERN},
                "date_to": {"type": "string", "pattern": _DATE_PATTERN},
                "currency": {"type": "string", "enum": ["RUB", "USD", "EUR"]},
                "title_query": {"type": "string"},
                "category": {
                    "type": "string",
                    "description": "Категория платежа, если есть. Мягкий фильтр",
                },
                "assignee_id": {
                    "type": "string",
                    "description": "id из find_people или me",
                },
                "group_by": {
                    "type": "string",
                    "enum": ["currency", "currency_assignee"],
                    "description": (
                        "currency — по валютам; currency_assignee — по валюте и ответственному"
                    ),
                },
                "basis": {
                    "type": "string",
                    "enum": ["period_sum", "run_rate"],
                    "description": (
                        "period_sum — сумма начислений со сроком в period (нужен period "
                        "или названный месяц). run_rate — средняя месячная нагрузка по "
                        "активным платежам (нормировка: ежемесячный x1, квартальный /3, "
                        "полугодовой /6, годовой /12). Для «сколько платим в месяц» без "
                        "названного месяца используй run_rate; для «в этом месяце», «в "
                        "октябре», «в ноябре» используй period_sum с period"
                    ),
                },
            },
            "required": ["status"],
        },
    },
}

FIND_RECURRING_PAYMENTS_SCHEMA: Final[dict[str, Any]] = {
    "type": "function",
    "function": {
        "name": "find_recurring_payments",
        "description": (
            "Найти регулярные платежи (подписки, аренда, сервисы): сумма, валюта, "
            "ответственный, ближайший срок. Используй для «кто отвечает за …», «какие "
            "подписки дороже …», «самый дорогой платёж»."
        ),
        "parameters": {
            "type": "object",
            "additionalProperties": False,
            "properties": {
                "title_query": {"type": "string", "description": "Слово из названия"},
                "category": {
                    "type": "string",
                    "description": "Категория платежа, если есть. Мягкий фильтр",
                },
                "currency": {"type": "string", "enum": ["RUB", "USD", "EUR"]},
                "min_amount": {"type": "number"},
                "max_amount": {"type": "number"},
                "assignee_id": {
                    "type": "string",
                    "description": "id из find_people или me",
                },
                "include_ended": {
                    "type": "boolean",
                    "description": "Включить завершённые платежи. По умолчанию нет",
                },
                "sort": {"type": "string", "enum": ["title", "amount_desc", "next_due"]},
                "limit": {"type": "integer", "minimum": 1, "maximum": 20},
            },
            "required": [],
        },
    },
}

FIND_PEOPLE_SCHEMA: Final[dict[str, Any]] = {
    "type": "function",
    "function": {
        "name": "find_people",
        "description": (
            "Найти сотрудников по имени или должности (персоне), получить их id для "
            "фильтра assignee_id и число активных платежей на них."
        ),
        "parameters": {
            "type": "object",
            "additionalProperties": False,
            "properties": {
                "query": {
                    "type": "string",
                    "description": "Имя, фамилия или должность, например Иван, бухгалтер",
                },
                "limit": {"type": "integer", "minimum": 1, "maximum": 20},
            },
            "required": [],
        },
    },
}

REPLY_WITHOUT_DATA_SCHEMA: Final[dict[str, Any]] = {
    "type": "function",
    "function": {
        "name": "reply_without_data",
        "description": (
            "Ответить без обращения к данным: приветствие, вопрос не про платежи, или "
            "нужно уточнение. Не используй, если вопрос про платежи, суммы, сроки или "
            "ответственных."
        ),
        "parameters": {
            "type": "object",
            "additionalProperties": False,
            "properties": {
                "reason": {
                    "type": "string",
                    "enum": ["greeting", "out_of_scope", "needs_clarification"],
                },
                "message": {
                    "type": "string",
                    "description": "Короткий ответ пользователю на русском, без цифр",
                },
            },
            "required": ["reason", "message"],
        },
    },
}

# tool name -> (pydantic model, flat schema)
MCP_TOOLS: Final[dict[str, tuple[type[_Args], dict[str, Any]]]] = {
    "find_occurrences": (FindOccurrencesArgs, FIND_OCCURRENCES_SCHEMA),
    "sum_occurrences": (SumOccurrencesArgs, SUM_OCCURRENCES_SCHEMA),
    "find_recurring_payments": (FindRecurringPaymentsArgs, FIND_RECURRING_PAYMENTS_SCHEMA),
    "find_people": (FindPeopleArgs, FIND_PEOPLE_SCHEMA),
}

LOCAL_TOOLS: Final[dict[str, tuple[type[_Args], dict[str, Any]]]] = {
    "reply_without_data": (ReplyWithoutDataArgs, REPLY_WITHOUT_DATA_SCHEMA),
}

ALL_TOOLS: Final[dict[str, tuple[type[_Args], dict[str, Any]]]] = {**MCP_TOOLS, **LOCAL_TOOLS}

# Schemas passed to OpenRouter on every request (4 MCP + 1 local), §7.1.
LLM_TOOL_SCHEMAS: Final[list[dict[str, Any]]] = [schema for _, schema in ALL_TOOLS.values()]
