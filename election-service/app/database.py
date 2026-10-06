import logging
import time
from collections.abc import Iterator

from sqlalchemy import create_engine, text
from sqlalchemy.exc import OperationalError
from sqlalchemy.orm import DeclarativeBase, Session, sessionmaker

from .config import settings

logger = logging.getLogger("election-service")


class Base(DeclarativeBase):
    pass


engine = create_engine(settings.database_url, pool_pre_ping=True)
SessionLocal = sessionmaker(bind=engine, autoflush=False)


def get_db() -> Iterator[Session]:
    """Dependencia de FastAPI: una sesión de base de datos por petición."""
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()


def wait_for_db() -> None:
    """Reintenta la conexión mientras PostgreSQL termina de arrancar."""
    for attempt in range(1, settings.db_connect_retries + 1):
        try:
            with engine.connect() as conn:
                conn.execute(text("SELECT 1"))
            logger.info("Conexión a la base de datos establecida")
            return
        except OperationalError:
            logger.warning(
                "Base de datos no disponible (intento %s/%s)",
                attempt,
                settings.db_connect_retries,
            )
            time.sleep(settings.db_connect_delay_seconds)
    raise RuntimeError("No fue posible conectarse a la base de datos")


def init_db() -> None:
    from . import models  # noqa: F401  (registra las tablas en Base.metadata)

    Base.metadata.create_all(bind=engine)
