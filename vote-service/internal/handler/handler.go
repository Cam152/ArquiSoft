// Package handler es la capa HTTP: lee la petición, llama a la capa de
// servicio y escribe la respuesta.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"slices"
	"strconv"

	"vote-service/internal/model"
	"vote-service/internal/service"
)

// Voting es la capa de servicio que usan los handlers.
type Voting interface {
	CastVote(ctx context.Context, authorization string, electionID, candidateID int) error
	Results(ctx context.Context, electionID int) (*model.Results, error)
}

// Handler contiene las dependencias de los handlers.
type Handler struct {
	voting      Voting
	corsOrigins []string
}

func New(voting Voting, corsOrigins []string) *Handler {
	return &Handler{voting: voting, corsOrigins: corsOrigins}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("POST /votes", h.castVote)
	mux.HandleFunc("GET /results/{election_id}", h.results)
	return cors(h.corsOrigins, mux)
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type voteRequest struct {
	ElectionID  *int `json:"election_id"`
	CandidateID *int `json:"candidate_id"`
}

func (h *Handler) castVote(w http.ResponseWriter, r *http.Request) {
	var body voteRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body); err != nil ||
		body.ElectionID == nil || body.CandidateID == nil {
		writeError(w, http.StatusUnprocessableEntity, "Se requieren election_id y candidate_id enteros")
		return
	}
	authorization := r.Header.Get("Authorization")
	if authorization == "" {
		writeError(w, http.StatusUnauthorized, "Falta el header Authorization")
		return
	}
	if err := h.voting.CastVote(r.Context(), authorization, *body.ElectionID, *body.CandidateID); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "recorded"})
}

func (h *Handler) results(w http.ResponseWriter, r *http.Request) {
	electionID, err := strconv.Atoi(r.PathValue("election_id"))
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "election_id debe ser un entero")
		return
	}
	results, err := h.voting.Results(r.Context(), electionID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, results)
}

// writeServiceError traduce un error de la capa de servicio a su respuesta HTTP.
func writeServiceError(w http.ResponseWriter, err error) {
	var rejected *model.VoterRejection
	switch {
	case errors.As(err, &rejected):
		status := http.StatusUnauthorized
		if rejected.Reason == model.AlreadyVoted {
			status = http.StatusConflict
		}
		writeError(w, status, rejected.Detail)
	case errors.Is(err, model.ErrElectionNotFound):
		writeError(w, http.StatusNotFound, "La elección no existe")
	case errors.Is(err, service.ErrElectionNotOpen):
		writeError(w, http.StatusConflict, "La elección no está abierta")
	case errors.Is(err, service.ErrCandidateNotInElection):
		writeError(w, http.StatusUnprocessableEntity, "El candidato no pertenece a la elección")
	case errors.Is(err, service.ErrElectionServiceUnavailable):
		writeError(w, http.StatusBadGateway, "El Election Service no responde")
	case errors.Is(err, service.ErrVoterServiceUnavailable):
		writeError(w, http.StatusBadGateway, "El Voter Service no responde")
	case errors.Is(err, service.ErrVoteNotStored):
		writeError(w, http.StatusInternalServerError, "No se pudo registrar el voto")
	case errors.Is(err, service.ErrResultsUnavailable):
		writeError(w, http.StatusInternalServerError, "No se pudieron calcular los resultados")
	default:
		log.Printf("error inesperado: %v", err)
		writeError(w, http.StatusInternalServerError, "Error interno")
	}
}

// cors permite los orígenes configurados y responde las peticiones previas OPTIONS.
func cors(allowed []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && slices.Contains(allowed, origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Add("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
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
