"""Shared helpers for the MCP tools: validation, envelopes, row shaping."""

from __future__ import annotations

import json
import logging
from collections.abc import Awaitable, Callable
from datetime import date
from decimal import Decimal
from typing import Any

from pydantic import ValidationError

from ...common import money, tool_specs
from ...common.actor import Actor
from ...common.errors import BadArgs, Forbidden, InvalidActor, NlqError
from ...common.sanitize import sanitize
from ..auth import current_actor
from ..db import load_sql
from ..fragments import category_col
from ..policy import Visibility, resolve_visibility
from ..state import AppState, get_state

log = logging.getLogger("nlq.mcp.tools")


def render_occurrences_base(state: AppState) -> str:
    """occurrences_base.sql with the code-chosen category column fragment."""
    return load_sql("occurrences_base.sql").format(
        category_col=category_col(state.schema.category_available)
    )


def category_filter(state: AppState, requested: str | None) -> tuple[str | None, bool]:
    """Resolve the optional category filter against B's schema (decision #3).

    Returns ``(bound_value, unavailable)``: when B has no category column the
    filter is dropped (``None``) and ``unavailable`` is True so the caller can
    surface a ``category_unavailable`` note instead of silently returning empty.
    """
    if not requested:
        return None, False
    if state.schema.category_available:
        return requested, False
    return None, True


async def compute_coverage(
    state: AppState, bounds: tuple[date, date] | None
) -> tuple[Any, bool]:
    """Per-active-payment horizon (section 5.4, decision #4).

    Returns ``(coverage_until, coverage_incomplete)``. Coverage is incomplete
    when any active payment has no periods, has none at all, or its last period
    is before the end of the requested window.
    """
    rows = await state.db.ro_query(load_sql("coverage.sql"))
    row = rows[0] if rows else None
    coverage_until = row["coverage_until"] if row else None
    any_missing = bool(row["any_without_periods"]) if row else False
    incomplete = bool(
        bounds is not None
        and (any_missing or coverage_until is None or bounds[1] > coverage_until)
    )
    return coverage_until, incomplete


def validate_args(model_name: str, raw: dict[str, Any]) -> Any:
    """Re-validate collected arguments through the pydantic model.

    This runs the cross-field validators (e.g. amount requires currency). Only
    non-None values are passed so optional fields keep their defaults.
    """
    model_cls, _ = tool_specs.ALL_TOOLS[model_name]
    provided = {k: v for k, v in raw.items() if v is not None}
    try:
        return model_cls.model_validate(provided)
    except ValidationError as exc:
        # Do not leak internals; summarise the first error's location.
        first = exc.errors()[0]
        loc = ".".join(str(p) for p in first.get("loc", ())) or "arguments"
        raise BadArgs(f"invalid argument: {loc}") from exc


def error_envelope(tool: str, err: NlqError) -> str:
    return json.dumps(
        {"ok": False, "tool": tool, "error": err.code, "message": err.message},
        ensure_ascii=False,
    )


def success_envelope(payload: dict[str, Any]) -> str:
    return json.dumps(payload, ensure_ascii=False)


async def run_tool(
    tool: str, fn: Callable[[AppState, Actor, Visibility], Awaitable[dict[str, Any]]]
) -> str:
    """Resolve identity, dispatch, and always return a JSON envelope string."""
    state = get_state()
    try:
        try:
            actor = current_actor()
        except InvalidActor as exc:
            raise Forbidden("invalid actor") from exc
        visibility = resolve_visibility(actor, state.settings.nlq_user_scope)
        payload = await fn(state, actor, visibility)
        payload = {"ok": True, "tool": tool, **payload}
        return success_envelope(payload)
    except NlqError as err:
        return error_envelope(tool, err)


def period_block(bounds: tuple[date, date] | None) -> dict[str, str] | None:
    if bounds is None:
        return None
    return {"from": bounds[0].isoformat(), "to": bounds[1].isoformat()}


def days_overdue(due: date, today: date, is_paid: bool) -> int | None:
    if is_paid or due >= today:
        return None
    return (today - due).days


def occurrence_row(record: Any, today: date) -> dict[str, Any]:
    amount = Decimal(record["amount"])
    currency = record["currency"]
    due: date = record["due_date"]
    is_paid: bool = record["is_paid"]
    return {
        "occurrence_id": record["occurrence_id"],
        "payment_id": record["payment_id"],
        "title": sanitize(record["title"]),
        "amount": money.to_amount_string(amount),
        "currency": currency,
        "display": money.display(amount, currency),
        "due_date": due.isoformat(),
        "days_overdue": days_overdue(due, today, is_paid),
        "is_paid": is_paid,
        "assignee_id": record["assignee_id"],
        "assignee_name": sanitize(record["assignee_name"]) if record["assignee_name"] else None,
    }
