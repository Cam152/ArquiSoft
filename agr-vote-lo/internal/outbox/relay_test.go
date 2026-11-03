package outbox

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"vote-service/internal/model"
)

// memStore simula la colección votes.
type memStore struct {
	mu    sync.Mutex
	votes []model.Vote
}

func (s *memStore) FindUnpublished(_ context.Context, limit int) ([]model.Vote, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []model.Vote
	for _, v := range s.votes {
		if !v.Published && len(out) < limit {
			out = append(out, v)
		}
	}
	return out, nil
}

func (s *memStore) MarkPublished(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.votes {
		if s.votes[i].ID == id {
			s.votes[i].Published = true
		}
	}
	return nil
}

func (s *memStore) pending() int {
	n, _ := s.FindUnpublished(context.Background(), 1000)
	return len(n)
}

// fakeBroker simula RabbitMQ: puede estar caído y puede bloquear la confirmación.
type fakeBroker struct {
	mu     sync.Mutex
	down   bool
	events []model.VoteRegistered
	gate   chan struct{} // si no es nil, Publish espera a que se cierre
}

func (b *fakeBroker) Publish(ctx context.Context, e model.VoteRegistered) error {
	if b.gate != nil {
		<-b.gate
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.down {
		return errors.New("broker caído")
	}
	b.events = append(b.events, e)
	return nil
}

func (b *fakeBroker) count() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.events)
}

func vote(id string) model.Vote {
	return model.Vote{ID: id, ElectionID: 1, CandidateID: 2, CandidateName: "Ana",
		CastAt: time.Date(2026, 11, 3, 14, 0, 0, 0, time.UTC), IdempotencyKey: "k-" + id}
}

func newRelay(s *memStore, b *fakeBroker) *Relay {
	r := NewRelay(s, b, time.Hour, 10)
	r.logPrint = func(string, ...any) {}
	return r
}

func TestDispatchPublishesAndMarks(t *testing.T) {
	store := &memStore{votes: []model.Vote{vote("a")}}
	broker := &fakeBroker{}
	relay := newRelay(store, broker)

	relay.Dispatch(store.votes[0])
	relay.Wait()

	if broker.count() != 1 || store.pending() != 0 {
		t.Fatalf("eventos %d, pendientes %d; se esperaba 1 y 0", broker.count(), store.pending())
	}
	got := broker.events[0]
	want := model.VoteRegistered{EventID: "a", Type: "VoteRegistered", Version: 1, ElectionID: 1, CandidateID: 2,
		OccurredAt: time.Date(2026, 11, 3, 14, 0, 0, 0, time.UTC)}
	if got != want {
		t.Fatalf("evento %+v, se esperaba %+v", got, want)
	}
}

// Si el broker falla, el voto queda pendiente y sale en la siguiente revisión.
func TestBrokerOutageIsRecoveredByFlush(t *testing.T) {
	store := &memStore{votes: []model.Vote{vote("a"), vote("b"), vote("c")}}
	broker := &fakeBroker{down: true}
	relay := newRelay(store, broker)

	for _, v := range store.votes {
		relay.Dispatch(v)
	}
	relay.Wait()
	if store.pending() != 3 {
		t.Fatalf("con el broker caído deben quedar 3 pendientes, hay %d", store.pending())
	}
	if n := relay.Flush(context.Background()); n != 0 {
		t.Fatalf("con el broker caído Flush publicó %d", n)
	}

	broker.down = false
	if n := relay.Flush(context.Background()); n != 3 {
		t.Fatalf("Flush publicó %d, se esperaba 3", n)
	}
	if store.pending() != 0 || broker.count() != 3 {
		t.Fatalf("pendientes %d, eventos %d", store.pending(), broker.count())
	}
	if n := relay.Flush(context.Background()); n != 0 {
		t.Fatalf("no debía republicar votos ya publicados, publicó %d", n)
	}
}

func TestFlushRespectsBatchSize(t *testing.T) {
	store := &memStore{}
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		store.votes = append(store.votes, vote(id))
	}
	relay := newRelay(store, &fakeBroker{})
	relay.batch = 2
	if n := relay.Flush(context.Background()); n != 2 {
		t.Fatalf("publicó %d, se esperaba la tanda de 2", n)
	}
}

// Mientras Dispatch está publicando un voto, Flush no lo publica otra vez.
func TestInFlightVoteIsNotPublishedTwice(t *testing.T) {
	store := &memStore{votes: []model.Vote{vote("a")}}
	broker := &fakeBroker{gate: make(chan struct{})}
	relay := newRelay(store, broker)

	relay.Dispatch(store.votes[0])
	// Dispatch está bloqueado en Publish; Flush debe saltarse el voto.
	done := make(chan int)
	go func() { done <- relay.Flush(context.Background()) }()
	select {
	case n := <-done:
		if n != 0 {
			t.Fatalf("Flush publicó %d votos en curso", n)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Flush se bloqueó esperando un voto en curso")
	}
	close(broker.gate)
	relay.Wait()
	if broker.count() != 1 {
		t.Fatalf("se publicaron %d eventos, se esperaba 1", broker.count())
	}
}

func TestRunStopsWithContext(t *testing.T) {
	store := &memStore{votes: []model.Vote{vote("a")}}
	broker := &fakeBroker{}
	relay := newRelay(store, broker)
	relay.interval = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() { relay.Run(ctx); close(stopped) }()

	deadline := time.Now().Add(2 * time.Second)
	for store.pending() != 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("Run no terminó al cancelar el contexto")
	}
	if broker.count() != 1 {
		t.Fatalf("Run publicó %d eventos, se esperaba 1", broker.count())
	}
}
