"""Visibility policy (§5.1 p.3).

Identity comes only from the verified actor, never from tool arguments. The
policy produces the two assignee parameters the SQL expects: the model's
explicit ``assignee_id`` filter and the server-imposed ``own`` filter.
"""

from __future__ import annotations

from dataclasses import dataclass
from uuid import UUID

from ..common.actor import Actor
from ..common.errors import Forbidden


@dataclass(frozen=True, slots=True)
class Visibility:
    """Effective visibility for a request."""

    scope: str  # "team" or "own"
    own_filter: UUID | None  # value bound to the server-side own filter param


def resolve_visibility(actor: Actor, user_scope: str) -> Visibility:
    if actor.role == "admin":
        return Visibility(scope="team", own_filter=None)
    if user_scope == "own":
        return Visibility(scope="own", own_filter=actor.sub)
    return Visibility(scope="team", own_filter=None)


def resolve_assignee(
    assignee_id: str | None, actor: Actor, visibility: Visibility
) -> UUID | None:
    """Turn the model's assignee filter into a UUID.

    "me" always resolves to the actor. Under ``own`` scope an explicit *other*
    assignee is forbidden; the names directory itself stays available (§5.6).
    """
    if assignee_id is None:
        return None
    if assignee_id == "me":
        return actor.sub
    try:
        target = UUID(assignee_id)
    except ValueError as exc:
        raise Forbidden("unknown assignee") from exc
    if visibility.scope == "own" and target != actor.sub:
        raise Forbidden("you can only filter by your own payments")
    return target
