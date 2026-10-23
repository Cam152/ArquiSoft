from fastapi import APIRouter, Depends, Query, Response, status
from sqlalchemy.orm import Session

from .. import services
from ..database import get_db
from ..models import ElectionStatus
from ..schemas import (
    CandidateCreate,
    CandidateOut,
    ElectionCreate,
    ElectionOut,
    ElectionSummary,
    ElectionUpdate,
)
from ..security import require_admin

router = APIRouter(prefix="/elections", tags=["elections"])


# ---------- Lectura (pública) ----------


@router.get("", response_model=list[ElectionSummary])
def list_elections(
    status_filter: ElectionStatus | None = Query(default=None, alias="status"),
    db: Session = Depends(get_db),
):
    return services.list_elections(db, status_filter)


@router.get("/{election_id}", response_model=ElectionOut)
def get_election(election_id: int, db: Session = Depends(get_db)):
    """Detalle de una elección con sus candidatos.

    Lo usa el Vote Service para validar que la elección esté abierta (`is_open`)
    y que el candidato exista.
    """
    return services.get_election(db, election_id)


@router.get("/{election_id}/candidates/{candidate_id}", response_model=CandidateOut)
def get_candidate(election_id: int, candidate_id: int, db: Session = Depends(get_db)):
    return services.get_candidate(db, election_id, candidate_id)


# ---------- Escritura (requiere X-API-Key) ----------


@router.post(
    "",
    response_model=ElectionOut,
    status_code=status.HTTP_201_CREATED,
    dependencies=[Depends(require_admin)],
)
def create_election(data: ElectionCreate, db: Session = Depends(get_db)):
    return services.create_election(db, data)


@router.patch(
    "/{election_id}",
    response_model=ElectionOut,
    dependencies=[Depends(require_admin)],
)
def update_election(election_id: int, data: ElectionUpdate, db: Session = Depends(get_db)):
    return services.update_election(db, election_id, data)


@router.delete(
    "/{election_id}",
    status_code=status.HTTP_204_NO_CONTENT,
    dependencies=[Depends(require_admin)],
)
def delete_election(election_id: int, db: Session = Depends(get_db)):
    services.delete_election(db, election_id)
    return Response(status_code=status.HTTP_204_NO_CONTENT)


@router.post(
    "/{election_id}/candidates",
    response_model=CandidateOut,
    status_code=status.HTTP_201_CREATED,
    dependencies=[Depends(require_admin)],
)
def add_candidate(election_id: int, data: CandidateCreate, db: Session = Depends(get_db)):
    return services.add_candidate(db, election_id, data)


@router.delete(
    "/{election_id}/candidates/{candidate_id}",
    status_code=status.HTTP_204_NO_CONTENT,
    dependencies=[Depends(require_admin)],
)
def remove_candidate(election_id: int, candidate_id: int, db: Session = Depends(get_db)):
    services.remove_candidate(db, election_id, candidate_id)
    return Response(status_code=status.HTTP_204_NO_CONTENT)


@router.post(
    "/{election_id}/activate",
    response_model=ElectionOut,
    dependencies=[Depends(require_admin)],
)
def activate_election(election_id: int, db: Session = Depends(get_db)):
    """Abre la elección (borrador -> activa). Requiere al menos 2 candidatos."""
    return services.activate_election(db, election_id)


@router.post(
    "/{election_id}/close",
    response_model=ElectionOut,
    dependencies=[Depends(require_admin)],
)
def close_election(election_id: int, db: Session = Depends(get_db)):
    """Cierra la elección (activa -> cerrada)."""
    return services.close_election(db, election_id)
