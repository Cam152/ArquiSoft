"""API GraphQL (solo lectura) pensada para el front-end.

Permite pedir una elección con sus candidatos anidados en una sola consulta y
elegir exactamente los campos que la pantalla necesita.
"""

from datetime import datetime

import strawberry
from fastapi import Depends
from sqlalchemy.orm import Session
from strawberry.fastapi import GraphQLRouter
from strawberry.types import Info

from . import models, services
from .database import get_db


@strawberry.type(name="Candidate")
class CandidateType:
    id: int
    election_id: int
    number: int
    name: str
    description: str | None


@strawberry.type(name="Election")
class ElectionType:
    id: int
    name: str
    description: str | None
    status: str
    starts_at: datetime | None
    ends_at: datetime | None
    created_at: datetime
    is_open: bool
    candidate_count: int
    candidates: list[CandidateType]


def _to_candidate(c: models.Candidate) -> CandidateType:
    return CandidateType(
        id=c.id,
        election_id=c.election_id,
        number=c.number,
        name=c.name,
        description=c.description,
    )


def _to_election(e: models.Election) -> ElectionType:
    return ElectionType(
        id=e.id,
        name=e.name,
        description=e.description,
        status=e.status,
        starts_at=e.starts_at,
        ends_at=e.ends_at,
        created_at=e.created_at,
        is_open=e.is_open,
        candidate_count=e.candidate_count,
        candidates=[_to_candidate(c) for c in e.candidates],
    )


@strawberry.type
class Query:
    @strawberry.field(description="Lista las elecciones, opcionalmente filtradas por estado (draft, active, closed).")
    def elections(self, info: Info, status: str | None = None) -> list[ElectionType]:
        db: Session = info.context["db"]
        return [_to_election(e) for e in services.list_elections(db, status)]

    @strawberry.field(description="Devuelve una elección con sus candidatos, o null si no existe.")
    def election(self, info: Info, id: int) -> ElectionType | None:
        db: Session = info.context["db"]
        try:
            return _to_election(services.get_election(db, id))
        except services.NotFoundError:
            return None


schema = strawberry.Schema(query=Query)


def get_context(db: Session = Depends(get_db)) -> dict:
    return {"db": db}


graphql_router = GraphQLRouter(schema, context_getter=get_context)
