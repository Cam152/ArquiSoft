// Package outbox garantiza que todo voto guardado termine publicado en
// agr-broker-as, aunque el broker falle justo después de guardarlo.
//
// El voto se guarda con published: false (en MongoDB la escritura de un
// documento es atómica). Luego:
//   - Dispatch publica su evento en segundo plano, apenas se guarda, y lo marca
//     published: true cuando el broker lo confirma.
//   - Run revisa cada cierto tiempo los votos que sigan en false y los vuelve a
//     publicar.
//
// La entrega es "al menos una vez": si el proceso cae entre la confirmación del
// broker y la marca, el evento sale de nuevo. Los consumidores descartan los
// repetidos por event_id.
package outbox

import (
	"context"
	"log"
	"sync"
	"time"

	"vote-service/internal/model"
)

// Store es la parte del repositorio que usa el outbox.
type Store interface {
	FindUnpublished(ctx context.Context, limit int) ([]model.Vote, error)
	MarkPublished(ctx context.Context, id string) error
}

// Publisher entrega un evento al broker y espera su confirmación.
type Publisher interface {
	Publish(ctx context.Context, event model.VoteRegistered) error
}

const publishTimeout = 10 * time.Second

// Relay publica los votos pendientes.
type Relay struct {
	store     Store
	publisher Publisher
	interval  time.Duration
	batch     int
	logPrint  func(format string, args ...any)

	// inFlight evita que Dispatch y Run publiquen el mismo voto a la vez.
	inFlight sync.Map
	wg       sync.WaitGroup
}

func NewRelay(store Store, publisher Publisher, interval time.Duration, batch int) *Relay {
	return &Relay{store: store, publisher: publisher, interval: interval, batch: batch, logPrint: log.Printf}
}

// Dispatch publica el voto en segundo plano; no bloquea la respuesta HTTP.
func (r *Relay) Dispatch(vote model.Vote) {
	if _, busy := r.inFlight.LoadOrStore(vote.ID, struct{}{}); busy {
		return
	}
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		defer r.inFlight.Delete(vote.ID)
		r.publish(context.Background(), vote)
	}()
}

// Run revisa los pendientes cada interval hasta que ctx termine.
func (r *Relay) Run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.Flush(ctx)
		}
	}
}

// Flush publica una tanda de votos pendientes. Se detiene en el primer fallo:
// si el broker está caído no tiene sentido intentar con el resto ahora.
// Devuelve cuántos publicó.
func (r *Relay) Flush(ctx context.Context) int {
	pending, err := r.store.FindUnpublished(ctx, r.batch)
	if err != nil {
		r.logPrint("outbox: no se pudieron leer los votos pendientes: %v", err)
		return 0
	}
	published := 0
	for _, vote := range pending {
		if ctx.Err() != nil {
			return published
		}
		if _, busy := r.inFlight.LoadOrStore(vote.ID, struct{}{}); busy {
			continue
		}
		ok := r.publish(ctx, vote)
		r.inFlight.Delete(vote.ID)
		if !ok {
			r.logPrint("outbox: %d votos siguen pendientes", len(pending)-published)
			return published
		}
		published++
	}
	return published
}

// Wait espera a que terminen las publicaciones en curso (apagado ordenado).
func (r *Relay) Wait() { r.wg.Wait() }

func (r *Relay) publish(parent context.Context, vote model.Vote) bool {
	ctx, cancel := context.WithTimeout(parent, publishTimeout)
	defer cancel()
	if err := r.publisher.Publish(ctx, model.EventFor(vote)); err != nil {
		// El voto queda en published: false y Run lo reintenta.
		r.logPrint("outbox: no se pudo publicar el evento %s: %v", vote.ID, err)
		return false
	}
	if err := r.store.MarkPublished(ctx, vote.ID); err != nil {
		// El evento ya salió; si se vuelve a publicar, el consumidor lo descarta.
		r.logPrint("outbox: evento %s publicado pero no se pudo marcar: %v", vote.ID, err)
		return false
	}
	return true
}
