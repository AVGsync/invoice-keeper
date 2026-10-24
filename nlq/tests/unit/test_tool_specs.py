from __future__ import annotations

from typing import Literal, Union, get_args, get_origin

import pytest
from pydantic import ValidationError

from nlq.common import tool_specs
from nlq.common.tool_specs import FindOccurrencesArgs, FindRecurringPaymentsArgs


def _literal_values(annotation: object) -> set[str] | None:
    """Return the string values of a Literal (through an Optional), else None."""
    if get_origin(annotation) is Literal:
        return {str(v) for v in get_args(annotation)}
    if get_origin(annotation) in (Union, type(int | str)):
        for arg in get_args(annotation):
            vals = _literal_values(arg)
            if vals is not None:
                return vals
    return None


@pytest.mark.parametrize("name", list(tool_specs.ALL_TOOLS))
def test_properties_match(name: str) -> None:
    model_cls, schema = tool_specs.ALL_TOOLS[name]
    schema_props = set(schema["function"]["parameters"]["properties"])
    model_fields = set(model_cls.model_fields)
    assert schema_props == model_fields


@pytest.mark.parametrize("name", list(tool_specs.ALL_TOOLS))
def test_required_match(name: str) -> None:
    model_cls, schema = tool_specs.ALL_TOOLS[name]
    schema_required = set(schema["function"]["parameters"].get("required", []))
    model_required = {n for n, f in model_cls.model_fields.items() if f.is_required()}
    assert schema_required == model_required


@pytest.mark.parametrize("name", list(tool_specs.ALL_TOOLS))
def test_enums_match(name: str) -> None:
    model_cls, schema = tool_specs.ALL_TOOLS[name]
    props = schema["function"]["parameters"]["properties"]
    for field_name, field in model_cls.model_fields.items():
        model_enum = _literal_values(field.annotation)
        schema_enum = props[field_name].get("enum")
        if schema_enum is not None:
            assert model_enum == set(schema_enum), field_name


@pytest.mark.parametrize("name", list(tool_specs.ALL_TOOLS))
def test_additional_properties_false(name: str) -> None:
    _, schema = tool_specs.ALL_TOOLS[name]
    assert schema["function"]["parameters"]["additionalProperties"] is False


def test_extra_field_rejected() -> None:
    with pytest.raises(ValidationError):
        FindOccurrencesArgs.model_validate({"status": "overdue", "user_id": "x"})


def test_amount_requires_currency() -> None:
    with pytest.raises(ValidationError):
        FindOccurrencesArgs.model_validate({"status": "overdue", "min_amount": 50})
    # With currency it is accepted.
    FindOccurrencesArgs.model_validate({"status": "overdue", "min_amount": 50, "currency": "EUR"})


def test_recurring_amount_requires_currency() -> None:
    with pytest.raises(ValidationError):
        FindRecurringPaymentsArgs.model_validate({"max_amount": 50})


def test_bad_assignee_rejected() -> None:
    with pytest.raises(ValidationError):
        FindOccurrencesArgs.model_validate({"status": "overdue", "assignee_id": "not-a-uuid"})


def test_me_assignee_ok() -> None:
    FindOccurrencesArgs.model_validate({"status": "overdue", "assignee_id": "me"})


def test_llm_schema_count() -> None:
    assert len(tool_specs.LLM_TOOL_SCHEMAS) == 11
