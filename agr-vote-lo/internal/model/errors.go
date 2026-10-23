package model

import "errors"

// ErrElectionNotFound lo devuelve el Election Service cuando la elección no existe.
var ErrElectionNotFound = errors.New("la elección no existe")

// RejectionReason es el motivo por el que el Voter Service rechazó al votante.
type RejectionReason int

const (
	InvalidToken RejectionReason = iota
	AlreadyVoted
)

// VoterRejection es un rechazo del Voter Service. Detail es su mensaje, que se
// devuelve tal cual al cliente.
type VoterRejection struct {
	Reason RejectionReason
	Detail string
}

func (e *VoterRejection) Error() string { return e.Detail }
