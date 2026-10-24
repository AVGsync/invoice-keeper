from __future__ import annotations

from uuid import UUID

import pytest

from nlq.common.actor import Actor
from nlq.common.errors import Forbidden
from nlq.mcp_server.fragments import STATUS_SQL, escape_like
from nlq.mcp_server.policy import resolve_assignee, resolve_visibility

ME = UUID("11110000-0000-0000-0000-000000000002")
OTHER = UUID("11110000-0000-0000-0000-000000000003")


def _actor(role: str) -> Actor:
    return Actor(sub=ME, role=role, ch="web")


def test_admin_sees_team() -> None:
    v = resolve_visibility(_actor("admin"), "own")
    assert v.scope == "team"
    assert v.own_filter is None


def test_user_team() -> None:
    v = resolve_visibility(_actor("user"), "team")
    assert v.scope == "team"
    assert v.own_filter is None


def test_user_own() -> None:
    v = resolve_visibility(_actor("user"), "own")
    assert v.scope == "own"
    assert v.own_filter == ME


def test_me_resolves_to_actor() -> None:
    v = resolve_visibility(_actor("user"), "own")
    assert resolve_assignee("me", _actor("user"), v) == ME


def test_other_forbidden_under_own() -> None:
    v = resolve_visibility(_actor("user"), "own")
    with pytest.raises(Forbidden):
        resolve_assignee(str(OTHER), _actor("user"), v)


def test_other_allowed_under_team() -> None:
    v = resolve_visibility(_actor("user"), "team")
    assert resolve_assignee(str(OTHER), _actor("user"), v) == OTHER


def test_none_assignee() -> None:
    v = resolve_visibility(_actor("user"), "team")
    assert resolve_assignee(None, _actor("user"), v) is None


def test_invalid_assignee_forbidden() -> None:
    v = resolve_visibility(_actor("user"), "team")
    with pytest.raises(Forbidden):
        resolve_assignee("garbage", _actor("user"), v)


def test_status_sql_covers_all() -> None:
    assert set(STATUS_SQL) == {"overdue", "upcoming", "unpaid", "paid", "any"}


def test_escape_like() -> None:
    assert escape_like("50%_a\\b") == "50\\%\\_a\\\\b"
