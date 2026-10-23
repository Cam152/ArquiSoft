// Package model tiene los tipos del dominio que comparten las demás capas.
package model

import "time"

// Vote es un voto anónimo: no lleva ningún dato del votante.
type Vote struct {
	ElectionID    int       `bson:"election_id"`
	CandidateID   int       `bson:"candidate_id"`
	CandidateName string    `bson:"candidate_name"`
	CastAt        time.Time `bson:"cast_at"`
}

// AuditEvent tampoco lleva datos del votante.
type AuditEvent struct {
	Type       string    `bson:"type"`
	ElectionID int       `bson:"election_id"`
	At         time.Time `bson:"at"`
}

// Candidate y Election son la parte de la respuesta del Election Service
// que necesita este servicio (GET /elections/{id}).
type Candidate struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Election struct {
	ID         int         `json:"id"`
	IsOpen     bool        `json:"is_open"`
	Candidates []Candidate `json:"candidates"`
}

// CandidateResult es el conteo de un candidato.
type CandidateResult struct {
	CandidateID   int     `json:"candidate_id"`
	CandidateName string  `json:"candidate_name"`
	Votes         int     `json:"votes"`
	Percentage    float64 `json:"percentage"`
}

// Results es el resultado de una elección, ordenado de más a menos votos.
type Results struct {
	ElectionID int               `json:"election_id"`
	TotalVotes int               `json:"total_votes"`
	Results    []CandidateResult `json:"results"`
}
