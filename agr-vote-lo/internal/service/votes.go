// Package service tiene las reglas de negocio del Vote Service: validar el voto,
// guardarlo una sola vez por Idempotency-Key y entregarlo al outbox. No sabe de
// HTTP, MongoDB ni RabbitMQ; usa las interfaces de este archivo.
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode/utf8"

	"vote-service/internal/model"
)

// Errores que devuelve Register, además de *model.ValidationError.
var (
	// ErrKeyReused: la Idempotency-Key ya se usó con un voto distinto.
	ErrKeyReused = errors.New("la Idempotency-Key ya se usó con otro voto")
	// ErrVoteNotStored: la base no aceptó el voto.
	ErrVoteNotStored = errors.New("no se pudo guardar el voto")
)

const (
	maxKeyLength           = 200
	maxCandidateNameLength = 150
)

// Repository es el acceso a agr-votes-db.
type Repository interface {
	// Insert devuelve model.ErrDuplicateKey si la Idempotency-Key ya existe.
	Insert(ctx context.Context, vote model.Vote) error
	FindByIdempotencyKey(ctx context.Context, key string) (*model.Vote, error)
}

// Dispatcher publica el evento de un voto recién guardado sin bloquear la
// respuesta. Lo implementa el outbox.
type Dispatcher interface {
	Dispatch(vote model.Vote)
}

// Votes registra votos.
type Votes struct {
	repo     Repository
	outbox   Dispatcher
	now      func() time.Time
	newID    func() (string, error)
	logPrint func(format string, args ...any)
}

func NewVotes(repo Repository, outbox Dispatcher) *Votes {
	return &Votes{repo: repo, outbox: outbox, now: time.Now, newID: randomID, logPrint: log.Printf}
}

// Register guarda el voto. Si la Idempotency-Key ya existía con el mismo voto
// (el gateway reintentó), no lo duplica y responde como si lo hubiera guardado.
func (s *Votes) Register(ctx context.Context, idempotencyKey string, in model.NewVote) error {
	key := strings.TrimSpace(idempotencyKey)
	if err := validate(key, in); err != nil {
		return err
	}
	id, err := s.newID()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrVoteNotStored, err)
	}
	vote := model.Vote{
		ID:            id,
		ElectionID:    in.ElectionID,
		CandidateID:   in.CandidateID,
		CandidateName: strings.TrimSpace(in.CandidateName),
		// Redondeada a la hora para que no se pueda cruzar con el momento en que
		// agr-voter-lo marcó al votante.
		CastAt:         s.now().UTC().Truncate(time.Hour),
		Published:      false,
		IdempotencyKey: key,
	}

	err = s.repo.Insert(ctx, vote)
	if errors.Is(err, model.ErrDuplicateKey) {
		return s.checkRetry(ctx, key, vote)
	}
	if err != nil {
		s.logPrint("no se pudo guardar el voto: %v", err)
		return ErrVoteNotStored
	}
	s.outbox.Dispatch(vote)
	return nil
}

// checkRetry confirma que la llave repetida corresponde al mismo voto. El
// evento de ese voto ya está en el outbox, así que no se vuelve a despachar.
func (s *Votes) checkRetry(ctx context.Context, key string, vote model.Vote) error {
	existing, err := s.repo.FindByIdempotencyKey(ctx, key)
	if err != nil || existing == nil {
		s.logPrint("no se pudo leer el voto repetido: %v", err)
		return ErrVoteNotStored
	}
	if existing.ElectionID != vote.ElectionID || existing.CandidateID != vote.CandidateID {
		return ErrKeyReused
	}
	return nil
}

func validate(key string, in model.NewVote) error {
	switch {
	case key == "":
		return &model.ValidationError{Detail: "Falta el header Idempotency-Key"}
	case len(key) > maxKeyLength:
		return &model.ValidationError{Detail: fmt.Sprintf("Idempotency-Key admite máximo %d caracteres", maxKeyLength)}
	case in.ElectionID <= 0 || in.CandidateID <= 0:
		return &model.ValidationError{Detail: "election_id y candidate_id deben ser enteros positivos"}
	case strings.TrimSpace(in.CandidateName) == "":
		return &model.ValidationError{Detail: "Falta candidate_name"}
	case utf8.RuneCountInString(in.CandidateName) > maxCandidateNameLength:
		return &model.ValidationError{Detail: fmt.Sprintf("candidate_name admite máximo %d caracteres", maxCandidateNameLength)}
	}
	return nil
}

// randomID genera el _id del voto: 12 bytes aleatorios en hexadecimal. No se usa
// un ObjectID de MongoDB porque este lleva la hora exacta de creación, y eso
// rompería el redondeo de cast_at (y viaja en el evento como event_id).
func randomID() (string, error) {
	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}
