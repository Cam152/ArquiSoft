package service_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"vote-service/internal/model"
	"vote-service/internal/service"
)

// Pruebas de la lógica sola: las tres dependencias son simuladas y cada una
// anota sus llamadas en calls, para comprobar el orden.

type fakes struct {
	calls []string

	election    *model.Election
	electionErr error
	markErr     error
	insertErr   error
	counts      map[int]int
	votes       []model.Vote
}

func (f *fakes) GetElection(context.Context, int) (*model.Election, error) {
	f.calls = append(f.calls, "get-election")
	return f.election, f.electionErr
}

func (f *fakes) MarkVoted(context.Context, string) error {
	f.calls = append(f.calls, "mark")
	return f.markErr
}

func (f *fakes) UnmarkVoted(context.Context, string) error {
	f.calls = append(f.calls, "unmark")
	return nil
}

func (f *fakes) InsertVote(_ context.Context, vote model.Vote) error {
	f.calls = append(f.calls, "insert-vote")
	if f.insertErr != nil {
		return f.insertErr
	}
	f.votes = append(f.votes, vote)
	return nil
}

func (f *fakes) InsertAudit(context.Context, model.AuditEvent) error {
	f.calls = append(f.calls, "insert-audit")
	return nil
}

func (f *fakes) CountByCandidate(context.Context, int) (map[int]int, error) {
	return f.counts, nil
}

func openElection() *model.Election {
	return &model.Election{ID: 1, IsOpen: true, Candidates: []model.Candidate{{ID: 1, Name: "Ana"}, {ID: 2, Name: "Luis"}}}
}

func newVoting(f *fakes) *service.Voting { return service.NewVoting(f, f, f) }

func TestCastVoteOrder(t *testing.T) {
	f := &fakes{election: openElection()}
	if err := newVoting(f).CastVote(context.Background(), "Bearer t", 1, 2); err != nil {
		t.Fatal(err)
	}
	if want := []string{"get-election", "mark", "insert-vote", "insert-audit"}; !slices.Equal(f.calls, want) {
		t.Fatalf("llamadas: %v, se esperaba %v", f.calls, want)
	}
	if len(f.votes) != 1 || f.votes[0].CandidateName != "Luis" {
		t.Fatalf("voto inesperado: %+v", f.votes)
	}
}

// Un voto inválido se rechaza antes de marcar al votante.
func TestInvalidVoteDoesNotMarkTheVoter(t *testing.T) {
	closed := openElection()
	closed.IsOpen = false
	cases := []struct {
		name        string
		f           *fakes
		candidateID int
		want        error
	}{
		{"elección inexistente", &fakes{electionErr: model.ErrElectionNotFound}, 1, model.ErrElectionNotFound},
		{"election service caído", &fakes{electionErr: errors.New("timeout")}, 1, service.ErrElectionServiceUnavailable},
		{"elección cerrada", &fakes{election: closed}, 1, service.ErrElectionNotOpen},
		{"candidato de otra elección", &fakes{election: openElection()}, 9, service.ErrCandidateNotInElection},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := newVoting(tc.f).CastVote(context.Background(), "Bearer t", 1, tc.candidateID)
			if !errors.Is(err, tc.want) {
				t.Fatalf("error %v, se esperaba %v", err, tc.want)
			}
			if !slices.Equal(tc.f.calls, []string{"get-election"}) {
				t.Fatalf("no debía llamar al Voter Service ni guardar: %v", tc.f.calls)
			}
		})
	}
}

func TestVoterRejectionIsReturnedAndNothingIsStored(t *testing.T) {
	rejection := &model.VoterRejection{Reason: model.AlreadyVoted, Detail: "El votante ya votó"}
	f := &fakes{election: openElection(), markErr: rejection}
	err := newVoting(f).CastVote(context.Background(), "Bearer t", 1, 2)

	var got *model.VoterRejection
	if !errors.As(err, &got) || got.Reason != model.AlreadyVoted || got.Detail != rejection.Detail {
		t.Fatalf("error inesperado: %v", err)
	}
	if slices.Contains(f.calls, "insert-vote") {
		t.Fatalf("no debía guardar el voto: %v", f.calls)
	}

	f = &fakes{election: openElection(), markErr: errors.New("timeout")}
	if err := newVoting(f).CastVote(context.Background(), "Bearer t", 1, 2); !errors.Is(err, service.ErrVoterServiceUnavailable) {
		t.Fatalf("error inesperado: %v", err)
	}
}

func TestFailedInsertUnmarksTheVoter(t *testing.T) {
	f := &fakes{election: openElection(), insertErr: errors.New("mongo caído")}
	err := newVoting(f).CastVote(context.Background(), "Bearer t", 1, 2)
	if !errors.Is(err, service.ErrVoteNotStored) {
		t.Fatalf("error inesperado: %v", err)
	}
	if want := []string{"get-election", "mark", "insert-vote", "unmark"}; !slices.Equal(f.calls, want) {
		t.Fatalf("llamadas: %v, se esperaba %v", f.calls, want)
	}
}

func TestResults(t *testing.T) {
	f := &fakes{election: openElection(), counts: map[int]int{2: 3, 1: 1}}
	got, err := newVoting(f).Results(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	want := []model.CandidateResult{
		{CandidateID: 2, CandidateName: "Luis", Votes: 3, Percentage: 75},
		{CandidateID: 1, CandidateName: "Ana", Votes: 1, Percentage: 25},
	}
	if got.TotalVotes != 4 || !slices.Equal(got.Results, want) {
		t.Fatalf("resultados inesperados: %+v", got)
	}

	empty, err := newVoting(&fakes{election: openElection()}).Results(context.Background(), 1)
	if err != nil || empty.TotalVotes != 0 || len(empty.Results) != 2 || empty.Results[0].Percentage != 0 {
		t.Fatalf("una elección sin votos debe listar sus candidatos en 0: %+v, %v", empty, err)
	}
}
