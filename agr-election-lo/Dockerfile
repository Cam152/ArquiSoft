FROM python:3.12-slim

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

WORKDIR /srv

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY app ./app

RUN useradd --create-home appuser
USER appuser

EXPOSE 8000
# Varios procesos para usar más de un núcleo (Lab 6). Con 4 workers y el pool
# por defecto (10 + 10) el servicio abre hasta 80 conexiones a PostgreSQL.
ENV UVICORN_WORKERS=4
CMD ["sh", "-c", "exec uvicorn app.main:app --host 0.0.0.0 --port 8000 --workers ${UVICORN_WORKERS}"]
