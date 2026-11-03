package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"vote-service/internal/model"
)

// fakeRepo reemplaza a MongoDB: guarda los votos por Idempotency-Key.
type fakeRepo struct {
	byKey     map[string]model.Vote
	insertErr error
	findErr   error
}

func newFakeRepo() *fakeRepo { return &fakeRepo{byKey: map[string]model.Vote{}} }

func (r *fakeRepo) Insert(_ context.Context, v model.Vote) error {
	if r.insertErr != nil {
		return r.insertErr
	}
	if _, ok := r.byKey[v.IdempotencyKey]; ok {
		return model.ErrDuplicateKey
	}
	r.byKey[v.IdempotencyKey] = v
	return nil
}

func (r *fakeRepo) FindByIdempotencyKey(_ context.Context, key string) (*model.Vote, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	v, ok := r.byKey[key]
	if !ok {
		return nil, nil
	}
	return &v, nil
}

type fakeOutbox struct{ dispatched []model.Vote }

func (o *fakeOutbox) Dispatch(v model.Vote) { o.dispatched = append(o.dispatched, v) }

func newService(repo *fakeRepo, outbox *fakeOutbox) *Votes {
	s := NewVotes(repo, outbox)
	s.now = func() time.Time { return time.Date(2026, 11, 3, 14, 37, 12, 0, time.FixedZone("COT", -5*3600)) }
	s.logPrint = func(string, ...any) {}
	return s
}

var valid = model.NewVote{ElectionID: 1, CandidateID: 2, CandidateName: "Luis Herrera"}

func TestRegisterStoresAnonymousUnpublishedVote(t *testing.T) {
	repo, outbox := newFakeRepo(), &fakeOutbox{}
	if err := newService(repo, outbox).Register(context.Background(), "key-1", valid); err != nil {
		t.Fatal(err)
	}
	v, ok := repo.byKey["key-1"]
	if !ok {
		t.Fatal("no se guardó el voto")
	}
	if v.ElectionID != 1 || v.CandidateID != 2 || v.CandidateName != "Luis Herrera" || v.Published {
		t.Fatalf("voto inesperado: %+v", v)
	}
	if want := time.Date(2026, 11, 3, 19, 0, 0, 0, time.UTC); !v.CastAt.Equal(want) || v.CastAt.Location() != time.UTC {
		t.Fatalf("cast_at = %v, se esperaba %v en UTC redondeado a la hora", v.CastAt, want)
	}
	if len(v.ID) != 24 {
		t.Fatalf("_id debe ser de 24 caracteres hexadecimales: %q", v.ID)
	}
	if len(outbox.dispatched) != 1 || outbox.dispatched[0].ID != v.ID {
		t.Fatalf("se esperaba despachar el voto al outbox: %+v", outbox.dispatched)
	}
}

func TestIDsAreRandom(t *testing.T) {
	a, _ := randomID()
	b, _ := randomID()
	if a == b {
		t.Fatalf("dos _id iguales: %s", a)
	}
}

func TestRetryWithSameKeyIsIdempotent(t *testing.T) {
	repo, outbox := newFakeRepo(), &fakeOutbox{}
	s := newService(repo, outbox)
	for i := 0; i < 3; i++ {
		if err := s.Register(context.Background(), "key-1", valid); err != nil {
			t.Fatalf("intento %d: %v", i, err)
		}
	}
	if len(repo.byKey) != 1 {
		t.Fatalf("se guardaron %d votos", len(repo.byKey))
	}
	if len(outbox.dispatched) != 1 {
		t.Fatalf("el evento se despachó %d veces", len(outbox.dispatched))
	}
}

func TestReusedKeyWithAnotherVoteIsRejected(t *testing.T) {
	repo, outbox := newFakeRepo(), &fakeOutbox{}
	s := newService(repo, outbox)
	if err := s.Register(context.Background(), "key-1", valid); err != nil {
		t.Fatal(err)
	}
	other := valid
	other.CandidateID = 3
	if err := s.Register(context.Background(), "key-1", other); !errors.Is(err, ErrKeyReused) {
		t.Fatalf("error %v, se esperaba ErrKeyReused", err)
	}
}

func TestValidation(t *testing.T) {
	cases := []struct {
		name string
		key  string
		in   model.NewVote
	}{
		{"sin llave", "", valid},
		{"llave en blanco", "   ", valid},
		{"llave muy larga", strings.Repeat("k", 201), valid},
		{"elección cero", "k", model.NewVote{ElectionID: 0, CandidateID: 2, CandidateName: "A"}},
		{"candidato negativo", "k", model.NewVote{ElectionID: 1, CandidateID: -1, CandidateName: "A"}},
		{"sin nombre", "k", model.NewVote{ElectionID: 1, CandidateID: 2, CandidateName: " "}},
		{"nombre muy largo", "k", model.NewVote{ElectionID: 1, CandidateID: 2, CandidateName: strings.Repeat("ñ", 151)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, outbox := newFakeRepo(), &fakeOutbox{}
			err := newService(repo, outbox).Register(context.Background(), tc.key, tc.in)
			var verr *model.ValidationError
			if !errors.As(err, &verr) || verr.Detail == "" {
				t.Fatalf("error %v, se esperaba ValidationError", err)
			}
			if len(repo.byKey) != 0 || len(outbox.dispatched) != 0 {
				t.Fatal("un voto inválido no debe guardarse ni publicarse")
			}
		})
	}
}

func TestStorageFailures(t *testing.T) {
	repo, outbox := newFakeRepo(), &fakeOutbox{}
	repo.insertErr = errors.New("mongo caído")
	if err := newService(repo, outbox).Register(context.Background(), "k", valid); !errors.Is(err, ErrVoteNotStored) {
		t.Fatalf("error %v, se esperaba ErrVoteNotStored", err)
	}
	if len(outbox.dispatched) != 0 {
		t.Fatal("no debe publicarse un voto que no se guardó")
	}

	// Llave repetida, pero no se puede leer el voto original.
	repo = newFakeRepo()
	repo.byKey["k"] = model.Vote{ElectionID: 1, CandidateID: 2}
	repo.findErr = errors.New("mongo caído")
	if err := newService(repo, outbox).Register(context.Background(), "k", valid); !errors.Is(err, ErrVoteNotStored) {
		t.Fatalf("error %v, se esperaba ErrVoteNotStored", err)
	}
}
