// Package service tiene la lógica del servicio: el flujo de votación y el
// cálculo de resultados. No sabe de HTTP ni de MongoDB; usa las interfaces de
// este archivo, que implementan los paquetes repository y client.
package service

import (
	"context"
	"errors"
	"log"
	"math"
	"slices"
	"sort"
	"time"

	"vote-service/internal/model"
)

// Errores que devuelve Voting, además de model.ErrElectionNotFound y *model.VoterRejection.
var (
	ErrElectionNotOpen            = errors.New("la elección no está abierta")
	ErrCandidateNotInElection     = errors.New("el candidato no pertenece a la elección")
	ErrElectionServiceUnavailable = errors.New("el Election Service no responde")
	ErrVoterServiceUnavailable    = errors.New("el Voter Service no responde")
	ErrVoteNotStored              = errors.New("no se pudo registrar el voto")
	ErrResultsUnavailable         = errors.New("no se pudieron calcular los resultados")
)

// VoteStore es el acceso a la Votes DB.
type VoteStore interface {
	InsertVote(ctx context.Context, vote model.Vote) error
	InsertAudit(ctx context.Context, event model.AuditEvent) error
	// CountByCandidate devuelve candidate_id -> número de votos.
	CountByCandidate(ctx context.Context, electionID int) (map[int]int, error)
}

// ElectionGateway consulta las elecciones en el Election Service.
type ElectionGateway interface {
	GetElection(ctx context.Context, id int) (*model.Election, error)
}

// VoterGateway marca y desmarca al votante en el Voter Service. authorization
// es el header Authorization del cliente, que identifica al votante.
type VoterGateway interface {
	MarkVoted(ctx context.Context, authorization string) error
	UnmarkVoted(ctx context.Context, authorization string) error
}

// Voting coordina el flujo de votación y calcula los resultados.
type Voting struct {
	store     VoteStore
	elections ElectionGateway
	voters    VoterGateway
}

func NewVoting(store VoteStore, elections ElectionGateway, voters VoterGateway) *Voting {
	return &Voting{store: store, elections: elections, voters: voters}
}

// CastVote sigue el orden del README (sección 7): valida la elección, marca al
// votante, guarda el voto y compensa si el guardado falla.
func (v *Voting) CastVote(ctx context.Context, authorization string, electionID, candidateID int) error {
	election, err := v.getElection(ctx, electionID)
	if err != nil {
		return err
	}
	if !election.IsOpen {
		return ErrElectionNotOpen
	}
	idx := slices.IndexFunc(election.Candidates, func(c model.Candidate) bool { return c.ID == candidateID })
	if idx < 0 {
		return ErrCandidateNotInElection
	}
	candidate := election.Candidates[idx]

	if err := v.voters.MarkVoted(ctx, authorization); err != nil {
		var rejected *model.VoterRejection
		if errors.As(err, &rejected) {
			return rejected
		}
		log.Printf("voter service no disponible: %v", err)
		return ErrVoterServiceUnavailable
	}

	// El votante ya quedó marcado: lo que sigue no se cancela si el cliente se desconecta.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()

	// La hora se redondea para que no se pueda cruzar con el momento del marcado.
	now := time.Now().UTC().Truncate(time.Hour)
	vote := model.Vote{ElectionID: election.ID, CandidateID: candidate.ID, CandidateName: candidate.Name, CastAt: now}
	if err := v.store.InsertVote(ctx, vote); err != nil {
		log.Printf("no se pudo guardar el voto: %v", err)
		if err := v.voters.UnmarkVoted(ctx, authorization); err != nil {
			log.Printf("falló la compensación (unmark-voted): %v", err)
		}
		return ErrVoteNotStored
	}

	event := model.AuditEvent{Type: "vote_registered", ElectionID: election.ID, At: now}
	if err := v.store.InsertAudit(ctx, event); err != nil {
		log.Printf("no se pudo registrar la auditoría: %v", err)
	}
	return nil
}

// Results devuelve el conteo y el porcentaje de cada candidato de la elección.
func (v *Voting) Results(ctx context.Context, electionID int) (*model.Results, error) {
	election, err := v.getElection(ctx, electionID)
	if err != nil {
		return nil, err
	}
	counts, err := v.store.CountByCandidate(ctx, electionID)
	if err != nil {
		log.Printf("no se pudieron contar los votos: %v", err)
		return nil, ErrResultsUnavailable
	}

	// Se parte de la lista de candidatos para que aparezcan los que tienen 0 votos.
	results := &model.Results{ElectionID: electionID, Results: make([]model.CandidateResult, 0, len(election.Candidates))}
	for _, c := range election.Candidates {
		results.TotalVotes += counts[c.ID]
		results.Results = append(results.Results, model.CandidateResult{CandidateID: c.ID, CandidateName: c.Name, Votes: counts[c.ID]})
	}
	for i := range results.Results {
		if results.TotalVotes > 0 {
			pct := float64(results.Results[i].Votes) * 100 / float64(results.TotalVotes)
			results.Results[i].Percentage = math.Round(pct*100) / 100
		}
	}
	sort.SliceStable(results.Results, func(i, j int) bool {
		return results.Results[i].Votes > results.Results[j].Votes
	})
	return results, nil
}

// getElection devuelve model.ErrElectionNotFound o ErrElectionServiceUnavailable si no la obtuvo.
func (v *Voting) getElection(ctx context.Context, id int) (*model.Election, error) {
	election, err := v.elections.GetElection(ctx, id)
	if errors.Is(err, model.ErrElectionNotFound) {
		return nil, model.ErrElectionNotFound
	}
	if err != nil {
		log.Printf("election service no disponible: %v", err)
		return nil, ErrElectionServiceUnavailable
	}
	return election, nil
}
