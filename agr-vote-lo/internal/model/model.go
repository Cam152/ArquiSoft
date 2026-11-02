// Package model tiene los tipos del dominio que comparten las demás capas.
package model

import (
	"errors"
	"time"
)

// Vote es un voto anónimo tal como se guarda en agr-votes-db. No lleva ningún
// dato del votante. Published es la marca del outbox: false hasta que el broker
// confirma el evento VoteRegistered.
type Vote struct {
	ID             string    `bson:"_id"`
	ElectionID     int       `bson:"election_id"`
	CandidateID    int       `bson:"candidate_id"`
	CandidateName  string    `bson:"candidate_name"`
	CastAt         time.Time `bson:"cast_at"`
	Published      bool      `bson:"published"`
	IdempotencyKey string    `bson:"idempotency_key"`
}

// NewVote es lo que envía el gateway en POST /votes.
type NewVote struct {
	ElectionID    int
	CandidateID   int
	CandidateName string
}

// Contrato del evento (README, sección 5.3).
const (
	EventType    = "VoteRegistered"
	EventVersion = 1
)

// VoteRegistered es el evento que se publica en el exchange agora.votes.
// Solo lleva lo necesario para contar: ningún dato del votante.
type VoteRegistered struct {
	EventID     string    `json:"event_id"`
	Type        string    `json:"type"`
	Version     int       `json:"version"`
	ElectionID  int       `json:"election_id"`
	CandidateID int       `json:"candidate_id"`
	OccurredAt  time.Time `json:"occurred_at"`
}

// EventFor arma el evento de un voto guardado. event_id es el _id del voto, así
// los consumidores pueden descartar repeticiones.
func EventFor(v Vote) VoteRegistered {
	return VoteRegistered{
		EventID:     v.ID,
		Type:        EventType,
		Version:     EventVersion,
		ElectionID:  v.ElectionID,
		CandidateID: v.CandidateID,
		OccurredAt:  v.CastAt.UTC(),
	}
}

// ErrDuplicateKey lo devuelve el repositorio cuando la Idempotency-Key ya existe.
var ErrDuplicateKey = errors.New("idempotency_key repetida")

// ValidationError es un dato de entrada inválido; Detail se devuelve al cliente.
type ValidationError struct{ Detail string }

func (e *ValidationError) Error() string { return e.Detail }
