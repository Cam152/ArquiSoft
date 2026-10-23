import enum
from datetime import datetime

from sqlalchemy import DateTime, ForeignKey, Integer, String, Text, UniqueConstraint
from sqlalchemy.orm import Mapped, mapped_column, relationship

from .database import Base
from .utils import as_utc, utcnow


class ElectionStatus(str, enum.Enum):
    DRAFT = "draft"
    ACTIVE = "active"
    CLOSED = "closed"


class Election(Base):
    __tablename__ = "elections"

    id: Mapped[int] = mapped_column(primary_key=True)
    name: Mapped[str] = mapped_column(String(200))
    description: Mapped[str | None] = mapped_column(Text, nullable=True)
    status: Mapped[str] = mapped_column(
        String(20), default=ElectionStatus.DRAFT.value, index=True
    )
    starts_at: Mapped[datetime | None] = mapped_column(
        DateTime(timezone=True), nullable=True
    )
    ends_at: Mapped[datetime | None] = mapped_column(
        DateTime(timezone=True), nullable=True
    )
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), default=utcnow
    )

    candidates: Mapped[list["Candidate"]] = relationship(
        back_populates="election",
        cascade="all, delete-orphan",
        order_by="Candidate.number",
    )

    @property
    def candidate_count(self) -> int:
        return len(self.candidates)

    @property
    def is_open(self) -> bool:
        """True si la elección está activa y la fecha actual está dentro de su ventana."""
        if self.status != ElectionStatus.ACTIVE.value:
            return False
        now = utcnow()
        if self.starts_at is not None and now < as_utc(self.starts_at):
            return False
        if self.ends_at is not None and now >= as_utc(self.ends_at):
            return False
        return True


class Candidate(Base):
    __tablename__ = "candidates"
    __table_args__ = (
        UniqueConstraint("election_id", "number", name="uq_candidate_number_per_election"),
    )

    id: Mapped[int] = mapped_column(primary_key=True)
    election_id: Mapped[int] = mapped_column(
        ForeignKey("elections.id", ondelete="CASCADE"), index=True
    )
    number: Mapped[int] = mapped_column(Integer)
    name: Mapped[str] = mapped_column(String(150))
    description: Mapped[str | None] = mapped_column(Text, nullable=True)

    election: Mapped["Election"] = relationship(back_populates="candidates")
