import logging
from contextlib import asynccontextmanager

from fastapi import Depends, FastAPI, HTTPException, Request
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse
from sqlalchemy import text
from sqlalchemy.orm import Session

from .config import settings
from .database import get_db, init_db, wait_for_db
from .graphql_api import graphql_router
from .routers import elections
from .seed import seed
from .services import ConflictError, InvalidDataError, NotFoundError

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s: %(message)s")


@asynccontextmanager
async def lifespan(app: FastAPI):
    wait_for_db()
    init_db()
    if settings.seed_data:
        seed()
    yield


app = FastAPI(
    title="Election Service",
    description=(
        "Gestiona elecciones y candidatos. Expone REST (para otros servicios y "
        "administración) y GraphQL en /graphql (para el front-end)."
    ),
    version="1.0.0",
    lifespan=lifespan,
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=settings.cors_origin_list,
    allow_methods=["*"],
    allow_headers=["*"],
)


@app.exception_handler(NotFoundError)
async def not_found_handler(request: Request, exc: NotFoundError):
    return JSONResponse(status_code=404, content={"detail": str(exc)})


@app.exception_handler(ConflictError)
async def conflict_handler(request: Request, exc: ConflictError):
    return JSONResponse(status_code=409, content={"detail": str(exc)})


@app.exception_handler(InvalidDataError)
async def invalid_data_handler(request: Request, exc: InvalidDataError):
    return JSONResponse(status_code=422, content={"detail": str(exc)})


@app.get("/health", tags=["health"])
def health(db: Session = Depends(get_db)):
    try:
        db.execute(text("SELECT 1"))
    except Exception:
        raise HTTPException(status_code=503, detail="Base de datos no disponible")
    return {"status": "ok", "service": "election-service"}


app.include_router(elections.router)
app.include_router(graphql_router, prefix="/graphql")
