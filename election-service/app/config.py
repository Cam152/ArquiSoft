from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """Configuración del servicio. Se lee de variables de entorno."""

    model_config = SettingsConfigDict(env_file=".env", extra="ignore")

    database_url: str = (
        "postgresql+psycopg2://elections:elections@elections-db:5432/elections"
    )
    admin_api_key: str = "dev-admin-key"
    cors_origins: str = "http://localhost:3000"
    seed_data: bool = True
    db_connect_retries: int = 30
    db_connect_delay_seconds: float = 1.0

    @property
    def cors_origin_list(self) -> list[str]:
        return [o.strip() for o in self.cors_origins.split(",") if o.strip()]


settings = Settings()
