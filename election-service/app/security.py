import secrets

from fastapi import Header, HTTPException, status

from .config import settings


def require_admin(x_api_key: str | None = Header(default=None)) -> None:
    """Protege las operaciones de escritura con una API key (header X-API-Key).

    La identidad de los votantes la maneja el Voter Service; esta llave solo
    protege la administración de elecciones en el prototipo.
    """
    expected = settings.admin_api_key.encode()
    received = (x_api_key or "").encode()
    if not secrets.compare_digest(received, expected):
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="API key inválida o ausente (header X-API-Key)",
        )
