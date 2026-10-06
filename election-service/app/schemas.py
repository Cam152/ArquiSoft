from datetime import datetime

from pydantic import BaseModel, ConfigDict, Field, field_validator, model_validator

from .models import ElectionStatus
from .utils import as_utc


def _check_window(starts_at: datetime | None, ends_at: datetime | None) -> None:
    if starts_at and ends_at and as_utc(ends_at) <= as_utc(starts_at):
        raise ValueError("ends_at debe ser posterior a starts_at")


# ---------- Entrada ----------


class CandidateCreate(BaseModel):
    model_config = ConfigDict(str_strip_whitespace=True)

    name: str = Field(min_length=1, max_length=150)
    description: str | None = Field(default=None, max_length=1000)


class ElectionCreate(BaseModel):
    model_config = ConfigDict(str_strip_whitespace=True)

    name: str = Field(min_length=3, max_length=200)
    description: str | None = Field(default=None, max_length=2000)
    starts_at: datetime | None = None
    ends_at: datetime | None = None

    @model_validator(mode="after")
    def validate_window(self):
        _check_window(self.starts_at, self.ends_at)
        return self


class ElectionUpdate(BaseModel):
    model_config = ConfigDict(str_strip_whitespace=True)

    name: str | None = Field(default=None, min_length=3, max_length=200)
    description: str | None = Field(default=None, max_length=2000)
    starts_at: datetime | None = None
    ends_at: datetime | None = None

    @field_validator("name")
    @classmethod
    def name_not_null(cls, value):
        if value is None:
            raise ValueError("name no puede ser nulo")
        return value

    @model_validator(mode="after")
    def validate_window(self):
        _check_window(self.starts_at, self.ends_at)
        return self


# ---------- Salida ----------


class CandidateOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: int
    election_id: int
    number: int
    name: str
    description: str | None = None


class ElectionSummary(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: int
    name: str
    description: str | None = None
    status: ElectionStatus
    starts_at: datetime | None = None
    ends_at: datetime | None = None
    created_at: datetime
    is_open: bool
    candidate_count: int


class ElectionOut(ElectionSummary):
    candidates: list[CandidateOut]
