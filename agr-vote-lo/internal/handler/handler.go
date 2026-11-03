// Package handler es la capa HTTP: lee la petición, llama a la capa de
// servicio y traduce sus errores a códigos HTTP.
//
// Solo lo llama agr-gateway-or (conector c6), nunca el navegador: por eso no
// hay CORS y POST /votes exige X-Service-Key.
package handler

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"vote-service/internal/model"
	"vote-service/internal/service"
)

// Registrar es la capa de servicio que usan los handlers.
type Registrar interface {
	Register(ctx context.Context, idempotencyKey string, vote model.NewVote) error
}

// Pinger comprueba la conexión con la base (health check).
type Pinger interface {
	Ping(ctx context.Context) error
}

// Handler contiene las dependencias de los handlers.
type Handler struct {
	votes      Registrar
	db         Pinger
	serviceKey [sha256.Size]byte
}

func New(votes Registrar, db Pinger, serviceKey string) *Handler {
	return &Handler{votes: votes, db: db, serviceKey: sha256.Sum256([]byte(serviceKey))}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.health)
	mux.Handle("POST /votes", h.requireServiceKey(http.HandlerFunc(h.castVote)))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "Ruta no encontrada")
	})
	return accessLog(mux)
}

// health responde 200 solo si MongoDB responde: sin base el servicio no puede
// guardar votos. El broker no se revisa aquí; si está caído, los votos se guardan
// igual y el outbox los publica cuando vuelva.
func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.db.Ping(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "agr-votes-db no responde")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type voteRequest struct {
	ElectionID    *int    `json:"election_id"`
	CandidateID   *int    `json:"candidate_id"`
	CandidateName *string `json:"candidate_name"`
}

func (h *Handler) castVote(w http.ResponseWriter, r *http.Request) {
	var body voteRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body); err != nil ||
		body.ElectionID == nil || body.CandidateID == nil || body.CandidateName == nil {
		writeError(w, http.StatusUnprocessableEntity, "Se requieren election_id, candidate_id (enteros) y candidate_name")
		return
	}
	vote := model.NewVote{ElectionID: *body.ElectionID, CandidateID: *body.CandidateID, CandidateName: *body.CandidateName}
	if err := h.votes.Register(r.Context(), r.Header.Get("Idempotency-Key"), vote); err != nil {
		writeServiceError(w, err)
		return
	}
	// 201 también cuando la llave se repite: para el gateway el voto quedó guardado.
	writeJSON(w, http.StatusCreated, map[string]string{"status": "recorded"})
}

// requireServiceKey compara X-Service-Key en tiempo constante.
func (h *Handler) requireServiceKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := sha256.Sum256([]byte(r.Header.Get("X-Service-Key")))
		if r.Header.Get("X-Service-Key") == "" || subtle.ConstantTimeCompare(got[:], h.serviceKey[:]) != 1 {
			writeError(w, http.StatusUnauthorized, "Llave de servicio inválida")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// writeServiceError traduce un error de la capa de servicio a su respuesta HTTP.
func writeServiceError(w http.ResponseWriter, err error) {
	var invalid *model.ValidationError
	switch {
	case errors.As(err, &invalid):
		writeError(w, http.StatusUnprocessableEntity, invalid.Detail)
	case errors.Is(err, service.ErrKeyReused):
		writeError(w, http.StatusConflict, "La Idempotency-Key ya se usó con otro voto")
	case errors.Is(err, service.ErrVoteNotStored):
		writeError(w, http.StatusInternalServerError, "No se pudo guardar el voto")
	default:
		log.Printf("error inesperado: %v", err)
		writeError(w, http.StatusInternalServerError, "Error interno")
	}
}

// statusRecorder guarda el código de respuesta para el log.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// accessLog registra método, ruta, código y duración. Nunca el cuerpo: aunque
// no lleva datos del votante, dice por quién se votó.
func accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if r.URL.Path != "/health" {
			log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Millisecond))
		}
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("no se pudo escribir la respuesta: %v", err)
	}
}

// writeError usa el formato común a todos los servicios: {"detail": "mensaje"}.
func writeError(w http.ResponseWriter, status int, detail string) {
	writeJSON(w, status, map[string]string{"detail": detail})
}
