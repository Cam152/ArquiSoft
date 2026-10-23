"""Reglas de negocio del Election Service.

Tanto la API REST como la API GraphQL usan estas funciones, de modo que las
reglas viven en un solo lugar.
"""

from sqlalchemy import func, select
from sqlalchemy.orm import Session, selectinload

from .models import Candidate, Election, ElectionStatus
from .schemas import CandidateCreate, ElectionCreate, ElectionUpdate
from .utils import as_utc

MIN_CANDIDATES_TO_ACTIVATE = 2


class NotFoundError(Exception):
    """El recurso no existe (HTTP 404)."""


class ConflictError(Exception):
    """La operación no es válida para el estado actual (HTTP 409)."""


class InvalidDataError(Exception):
    """Los datos enviados no son válidos (HTTP 422)."""


# ---------- Consultas ----------


def list_elections(db: Session, status: ElectionStatus | str | None = None) -> list[Election]:
    stmt = (
        select(Election)
        .options(selectinload(Election.candidates))
        .order_by(Election.id)
    )
    if status is not None:
        try:
            wanted = ElectionStatus(status)
        except ValueError:
            valid = ", ".join(s.value for s in ElectionStatus)
            raise InvalidDataError(f"Estado inválido '{status}'. Valores válidos: {valid}")
        stmt = stmt.where(Election.status == wanted.value)
    return list(db.scalars(stmt))


def get_election(db: Session, election_id: int) -> Election:
    stmt = (
        select(Election)
        .options(selectinload(Election.candidates))
        .where(Election.id == election_id)
    )
    election = db.scalars(stmt).first()
    if election is None:
        raise NotFoundError(f"La elección {election_id} no existe")
    return election


def get_candidate(db: Session, election_id: int, candidate_id: int) -> Candidate:
    get_election(db, election_id)
    candidate = db.scalars(
        select(Candidate).where(
            Candidate.id == candidate_id, Candidate.election_id == election_id
        )
    ).first()
    if candidate is None:
        raise NotFoundError(
            f"El candidato {candidate_id} no existe en la elección {election_id}"
        )
    return candidate


# ---------- Elecciones ----------


def create_election(db: Session, data: ElectionCreate) -> Election:
    election = Election(
        name=data.name,
        description=data.description,
        starts_at=data.starts_at,
        ends_at=data.ends_at,
        status=ElectionStatus.DRAFT.value,
    )
    db.add(election)
    db.commit()
    return get_election(db, election.id)


def update_election(db: Session, election_id: int, data: ElectionUpdate) -> Election:
    election = get_election(db, election_id)
    _require_draft(election, "modificar la elección")

    for field, value in data.model_dump(exclude_unset=True).items():
        setattr(election, field, value)

    if (
        election.starts_at is not None
        and election.ends_at is not None
        and as_utc(election.ends_at) <= as_utc(election.starts_at)
    ):
        db.rollback()
        raise InvalidDataError("ends_at debe ser posterior a starts_at")

    db.commit()
    return get_election(db, election_id)


def delete_election(db: Session, election_id: int) -> None:
    election = get_election(db, election_id)
    _require_draft(election, "eliminar la elección")
    db.delete(election)
    db.commit()


def activate_election(db: Session, election_id: int) -> Election:
    election = get_election(db, election_id)
    if election.status != ElectionStatus.DRAFT.value:
        raise ConflictError(
            f"La elección está en estado '{election.status}'; "
            "solo una elección en borrador puede abrirse"
        )
    if len(election.candidates) < MIN_CANDIDATES_TO_ACTIVATE:
        raise ConflictError(
            f"Se requieren al menos {MIN_CANDIDATES_TO_ACTIVATE} candidatos "
            "para abrir la elección"
        )
    election.status = ElectionStatus.ACTIVE.value
    db.commit()
    return get_election(db, election_id)


def close_election(db: Session, election_id: int) -> Election:
    election = get_election(db, election_id)
    if election.status != ElectionStatus.ACTIVE.value:
        raise ConflictError(
            f"La elección está en estado '{election.status}'; "
            "solo una elección activa puede cerrarse"
        )
    election.status = ElectionStatus.CLOSED.value
    db.commit()
    return get_election(db, election_id)


# ---------- Candidatos ----------


def add_candidate(db: Session, election_id: int, data: CandidateCreate) -> Candidate:
    election = get_election(db, election_id)
    _require_draft(election, "agregar candidatos")

    if any(c.name.casefold() == data.name.casefold() for c in election.candidates):
        raise ConflictError(f"Ya existe un candidato llamado '{data.name}' en esta elección")

    last_number = db.scalar(
        select(func.max(Candidate.number)).where(Candidate.election_id == election_id)
    )
    candidate = Candidate(
        election_id=election_id,
        number=(last_number or 0) + 1,
        name=data.name,
        description=data.description,
    )
    db.add(candidate)
    db.commit()
    db.refresh(candidate)
    return candidate


def remove_candidate(db: Session, election_id: int, candidate_id: int) -> None:
    election = get_election(db, election_id)
    _require_draft(election, "quitar candidatos")
    candidate = get_candidate(db, election_id, candidate_id)
    db.delete(candidate)
    db.commit()


# ---------- Auxiliares ----------


def _require_draft(election: Election, action: str) -> None:
    if election.status != ElectionStatus.DRAFT.value:
        raise ConflictError(
            f"No se puede {action}: la elección está en estado '{election.status}'. "
            "Solo las elecciones en borrador se pueden editar"
        )
