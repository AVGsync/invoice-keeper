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
    "today",
    "this_week",
    "last_week",
    "next_7_days",
    "this_month",
    "last_month",
    "next_month",
    "this_year",
    "custom",
]
_PERIOD_ENUM = [
    "today",
    "this_week",
    "last_week",
    "next_7_days",
    "this_month",
    "last_month",
    "next_month",
    "this_year",
    "custom",
]

_DATE_PATTERN = r"^\d{4}-\d{2}-\d{2}$"
_STATUS_ENUM = ["overdue", "upcoming", "unpaid", "paid", "any"]
_CURRENCY_ENUM = ["RUB", "USD", "EUR"]


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
    frequency: Literal["weekly", "monthly"] | None = None
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


class GetPaymentDetailsArgs(_Args):
    title_query: str = Field(min_length=1, max_length=64)
    assignee_id: str | None = None
    include_ended: bool = False

    @model_validator(mode="after")
    def _check(self) -> GetPaymentDetailsArgs:
        _validate_assignee(self.assignee_id)
        return self


class MonthlyBreakdownArgs(_Args):
    status: Status
    months: int = Field(default=6, ge=1, le=12)
    date_from: str | None = Field(default=None, pattern=_DATE_PATTERN)
    date_to: str | None = Field(default=None, pattern=_DATE_PATTERN)
    currency: Currency | None = None
    assignee_id: str | None = None
    title_query: str | None = Field(default=None, min_length=1, max_length=64)

    @model_validator(mode="after")
    def _check(self) -> MonthlyBreakdownArgs:
        if (self.date_from is None) != (self.date_to is None):
            raise ValueError("date_from and date_to must be given together")
        _validate_assignee(self.assignee_id)
        return self


class ComparePeriodsArgs(_Args):
    status: Status
    period_a: PeriodArg
    a_from: str | None = Field(default=None, pattern=_DATE_PATTERN)
    a_to: str | None = Field(default=None, pattern=_DATE_PATTERN)
    period_b: PeriodArg
    b_from: str | None = Field(default=None, pattern=_DATE_PATTERN)
    b_to: str | None = Field(default=None, pattern=_DATE_PATTERN)
    currency: Currency | None = None
    assignee_id: str | None = None
    title_query: str | None = Field(default=None, min_length=1, max_length=64)

    @model_validator(mode="after")
    def _check(self) -> ComparePeriodsArgs:
        if self.period_a == "custom" and (self.a_from is None or self.a_to is None):
            raise ValueError("period_a=custom requires a_from and a_to")
        if self.period_b == "custom" and (self.b_from is None or self.b_to is None):
            raise ValueError("period_b=custom requires b_from and b_to")
        _validate_assignee(self.assignee_id)
        return self


class WorkloadArgs(_Args):
    group_by: Literal["assignee", "persona"]
    status: Status = "unpaid"
    period: PeriodArg | None = None
    date_from: str | None = Field(default=None, pattern=_DATE_PATTERN)
    date_to: str | None = Field(default=None, pattern=_DATE_PATTERN)
    currency: Currency | None = None

    @model_validator(mode="after")
    def _check(self) -> WorkloadArgs:
        if self.period != "custom" and (self.date_from is not None or self.date_to is not None):
            raise ValueError("date_from/date_to are only allowed with period=custom")
        return self


class ListPersonasArgs(_Args):
    query: str | None = Field(default=None, min_length=1, max_length=64)
    limit: int | None = Field(default=None, ge=1, le=20)


class RecallConversationArgs(_Args):
    query: str | None = Field(default=None, min_length=1, max_length=100)
    limit: int = Field(default=3, ge=1, le=5)


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
                    "enum": _PERIOD_ENUM,
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
                "period": {"type": "string", "enum": _PERIOD_ENUM},
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
                        "активным платежам (нормировка по периодичности: месячный делится "
                        "на interval, недельный приводится к месяцу: x52/12/interval). Для "
                        "«сколько платим в месяц» без названного месяца используй run_rate; "
                        "для «в этом месяце», «в октябре», «в ноябре» используй period_sum с period"
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
                "frequency": {
                    "type": "string",
                    "enum": ["weekly", "monthly"],
                    "description": "Периодичность: weekly — еженедельные, monthly — ежемесячные",
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

GET_PAYMENT_DETAILS_SCHEMA: Final[dict[str, Any]] = {
    "type": "function",
    "function": {
        "name": "get_payment_details",
        "description": (
            "Подробности по одному регулярному платежу: сумма, периодичность, ответственный, "
            "ближайший срок, когда платили в последний раз, сколько просрочено и сколько раз "
            "оплачено за год. Используй для «подробнее про …», «когда последний раз платили за …»."
        ),
        "parameters": {
            "type": "object",
            "additionalProperties": False,
            "properties": {
                "title_query": {"type": "string", "description": "Слово из названия платежа"},
                "assignee_id": {"type": "string", "description": "id из find_people или me"},
                "include_ended": {"type": "boolean", "description": "Включить завершённые"},
            },
            "required": ["title_query"],
        },
    },
}

MONTHLY_BREAKDOWN_SCHEMA: Final[dict[str, Any]] = {
    "type": "function",
    "function": {
        "name": "monthly_breakdown",
        "description": (
            "Суммы по месяцам отдельно по каждой валюте: динамика расходов. Используй для "
            "«по месяцам», «как менялись расходы», «динамика»."
        ),
        "parameters": {
            "type": "object",
            "additionalProperties": False,
            "properties": {
                "status": {"type": "string", "enum": _STATUS_ENUM},
                "months": {
                    "type": "integer",
                    "minimum": 1,
                    "maximum": 12,
                    "description": "Сколько последних месяцев (по умолчанию 6)",
                },
                "date_from": {"type": "string", "pattern": _DATE_PATTERN},
                "date_to": {"type": "string", "pattern": _DATE_PATTERN},
                "currency": {"type": "string", "enum": _CURRENCY_ENUM},
                "assignee_id": {"type": "string", "description": "id из find_people или me"},
                "title_query": {"type": "string", "description": "Слово из названия платежа"},
            },
            "required": ["status"],
        },
    },
}

COMPARE_PERIODS_SCHEMA: Final[dict[str, Any]] = {
    "type": "function",
    "function": {
        "name": "compare_periods",
        "description": (
            "Сравнить суммы двух периодов по каждой валюте (стало больше или меньше). "
            "period_a — более ранний период, period_b — более поздний."
        ),
        "parameters": {
            "type": "object",
            "additionalProperties": False,
            "properties": {
                "status": {"type": "string", "enum": _STATUS_ENUM},
                "period_a": {"type": "string", "enum": _PERIOD_ENUM},
                "a_from": {"type": "string", "pattern": _DATE_PATTERN},
                "a_to": {"type": "string", "pattern": _DATE_PATTERN},
                "period_b": {"type": "string", "enum": _PERIOD_ENUM},
                "b_from": {"type": "string", "pattern": _DATE_PATTERN},
                "b_to": {"type": "string", "pattern": _DATE_PATTERN},
                "currency": {"type": "string", "enum": _CURRENCY_ENUM},
                "assignee_id": {"type": "string", "description": "id из find_people или me"},
                "title_query": {"type": "string", "description": "Слово из названия платежа"},
            },
            "required": ["status", "period_a", "period_b"],
        },
    },
}

WORKLOAD_SCHEMA: Final[dict[str, Any]] = {
    "type": "function",
    "function": {
        "name": "workload",
        "description": (
            "Нагрузка по платежам: сколько и на какую сумму приходится на каждого сотрудника "
            "или должность, отдельно по валютам. Используй для «у кого больше всего платежей», "
            "«нагрузка по сотрудникам/должностям»."
        ),
        "parameters": {
            "type": "object",
            "additionalProperties": False,
            "properties": {
                "group_by": {
                    "type": "string",
                    "enum": ["assignee", "persona"],
                    "description": "assignee — по сотрудникам; persona — по должностям",
                },
                "status": {"type": "string", "enum": _STATUS_ENUM},
                "period": {"type": "string", "enum": _PERIOD_ENUM},
                "date_from": {"type": "string", "pattern": _DATE_PATTERN},
                "date_to": {"type": "string", "pattern": _DATE_PATTERN},
                "currency": {"type": "string", "enum": _CURRENCY_ENUM},
            },
            "required": ["group_by"],
        },
    },
}

LIST_PERSONAS_SCHEMA: Final[dict[str, Any]] = {
    "type": "function",
    "function": {
        "name": "list_personas",
        "description": (
            "Список должностей (персон): сколько сотрудников и активных платежей на каждой. "
            "Используй для «какие есть должности»."
        ),
        "parameters": {
            "type": "object",
            "additionalProperties": False,
            "properties": {
                "query": {"type": "string", "description": "Слово из названия должности"},
                "limit": {"type": "integer", "minimum": 1, "maximum": 20},
            },
            "required": [],
        },
    },
}

RECALL_CONVERSATION_SCHEMA: Final[dict[str, Any]] = {
    "type": "function",
    "function": {
        "name": "recall_conversation",
        "description": (
            "Вспомнить прошлые вопросы и ответы этого диалога, если нужного нет в показанной "
            "истории. Возвращает краткие записи прошлых ходов (вопрос, ответ, факты, числа)."
        ),
        "parameters": {
            "type": "object",
            "additionalProperties": False,
            "properties": {
                "query": {"type": "string", "description": "Слова для поиска по прошлым ходам"},
                "limit": {"type": "integer", "minimum": 1, "maximum": 5},
            },
            "required": [],
        },
    },
}

# tool name -> (pydantic model, flat schema)
MCP_TOOLS: Final[dict[str, tuple[type[_Args], dict[str, Any]]]] = {
    "find_occurrences": (FindOccurrencesArgs, FIND_OCCURRENCES_SCHEMA),
    "sum_occurrences": (SumOccurrencesArgs, SUM_OCCURRENCES_SCHEMA),
    "find_recurring_payments": (FindRecurringPaymentsArgs, FIND_RECURRING_PAYMENTS_SCHEMA),
    "find_people": (FindPeopleArgs, FIND_PEOPLE_SCHEMA),
    "get_payment_details": (GetPaymentDetailsArgs, GET_PAYMENT_DETAILS_SCHEMA),
    "monthly_breakdown": (MonthlyBreakdownArgs, MONTHLY_BREAKDOWN_SCHEMA),
    "compare_periods": (ComparePeriodsArgs, COMPARE_PERIODS_SCHEMA),
    "workload": (WorkloadArgs, WORKLOAD_SCHEMA),
    "list_personas": (ListPersonasArgs, LIST_PERSONAS_SCHEMA),
}

LOCAL_TOOLS: Final[dict[str, tuple[type[_Args], dict[str, Any]]]] = {
    "reply_without_data": (ReplyWithoutDataArgs, REPLY_WITHOUT_DATA_SCHEMA),
    "recall_conversation": (RecallConversationArgs, RECALL_CONVERSATION_SCHEMA),
}

# Local tools that still return data (executed in-process, not via MCP), §D.6.
LOCAL_DATA_TOOLS: Final[frozenset[str]] = frozenset({"recall_conversation"})

ALL_TOOLS: Final[dict[str, tuple[type[_Args], dict[str, Any]]]] = {**MCP_TOOLS, **LOCAL_TOOLS}

# Schemas passed to OpenRouter on every request (9 MCP + 2 local), §7.1 / §D.7.
LLM_TOOL_SCHEMAS: Final[list[dict[str, Any]]] = [schema for _, schema in ALL_TOOLS.values()]
