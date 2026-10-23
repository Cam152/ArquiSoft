"""Datos de ejemplo para poder probar el prototipo sin crear nada a mano."""

import logging

from sqlalchemy import func, select

from . import services
from .database import SessionLocal
from .models import Election
from .schemas import CandidateCreate, ElectionCreate

logger = logging.getLogger("election-service")


def _create(db, name, description, candidates):
    election = services.create_election(db, ElectionCreate(name=name, description=description))
    for cand_name, cand_description in candidates:
        services.add_candidate(
            db, election.id, CandidateCreate(name=cand_name, description=cand_description)
        )
    return election


def seed() -> None:
    with SessionLocal() as db:
        if db.scalar(select(func.count(Election.id))):
            return

        active = _create(
            db,
            "Elección de representante estudiantil 2026",
            "Se elige un representante estudiantil ante el consejo de facultad.",
            [
                ("Ana Torres", "Más cupos en laboratorios y salas de estudio."),
                ("Luis Herrera", "Fortalecer el apoyo a prácticas y pasantías."),
                ("Camila Duarte", "Mejorar la conectividad y los espacios de cómputo."),
            ],
        )
        services.activate_election(db, active.id)

        _create(
            db,
            "Consulta sobre el horario de laboratorios",
            "Consulta en borrador: aún no está abierta a votación.",
            [("Horario diurno", None), ("Horario nocturno", None)],
        )

        closed = _create(
            db,
            "Elección de monitores 2025-II",
            "Elección ya finalizada.",
            [("Equipo A", None), ("Equipo B", None)],
        )
        services.activate_election(db, closed.id)
        services.close_election(db, closed.id)

    logger.info("Datos de ejemplo cargados")
