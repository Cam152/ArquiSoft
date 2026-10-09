package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"time"
)

// App contiene las dependencias de los handlers.
type App struct {
	cfg     Config
	store   Store
	clients *Clients
}

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", a.health)
	mux.HandleFunc("POST /votes", a.castVote)
	mux.HandleFunc("GET /results/{election_id}", a.results)
	return cors(a.cfg.CORSOrigins, mux)
}

func (a *App) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type voteRequest struct {
	ElectionID  *int `json:"election_id"`
	CandidateID *int `json:"candidate_id"`
}

// castVote sigue el orden del README (sección 7): valida la elección, marca al
// votante, guarda el voto y compensa si el guardado falla.
func (a *App) castVote(w http.ResponseWriter, r *http.Request) {
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

	election, ok := a.fetchElection(w, r, *body.ElectionID)
	if !ok {
		return
	}
	if !election.IsOpen {
		writeError(w, http.StatusConflict, "La elección no está abierta")
		return
	}
	idx := slices.IndexFunc(election.Candidates, func(c Candidate) bool { return c.ID == *body.CandidateID })
	if idx < 0 {
		writeError(w, http.StatusUnprocessableEntity, "El candidato no pertenece a la elección")
		return
	}
	candidate := election.Candidates[idx]

	if err := a.clients.MarkVoted(r.Context(), authorization); err != nil {
		var rejected *voterError
		if errors.As(err, &rejected) {
			writeError(w, rejected.Status, rejected.Detail)
			return
		}
		log.Printf("voter service no disponible: %v", err)
		writeError(w, http.StatusBadGateway, "El Voter Service no responde")
		return
	}

	// El votante ya quedó marcado: lo que sigue no se cancela si el cliente se desconecta.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 10*time.Second)
	defer cancel()

	// La hora se redondea para que no se pueda cruzar con el momento del marcado.
	now := time.Now().UTC().Truncate(time.Hour)
	vote := Vote{ElectionID: election.ID, CandidateID: candidate.ID, CandidateName: candidate.Name, CastAt: now}
	if err := a.store.InsertVote(ctx, vote); err != nil {
		log.Printf("no se pudo guardar el voto: %v", err)
		if err := a.clients.UnmarkVoted(ctx, authorization); err != nil {
			log.Printf("falló la compensación (unmark-voted): %v", err)
		}
		writeError(w, http.StatusInternalServerError, "No se pudo registrar el voto")
		return
	}

	event := AuditEvent{Type: "vote_registered", ElectionID: election.ID, At: now}
	if err := a.store.InsertAudit(ctx, event); err != nil {
		log.Printf("no se pudo registrar la auditoría: %v", err)
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "recorded"})
}

type candidateResult struct {
	CandidateID   int     `json:"candidate_id"`
	CandidateName string  `json:"candidate_name"`
	Votes         int     `json:"votes"`
	Percentage    float64 `json:"percentage"`
}

type resultsResponse struct {
	ElectionID int               `json:"election_id"`
	TotalVotes int               `json:"total_votes"`
	Results    []candidateResult `json:"results"`
}

func (a *App) results(w http.ResponseWriter, r *http.Request) {
	electionID, err := strconv.Atoi(r.PathValue("election_id"))
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "election_id debe ser un entero")
		return
	}
	election, ok := a.fetchElection(w, r, electionID)
	if !ok {
		return
	}
	counts, err := a.store.CountByCandidate(r.Context(), electionID)
	if err != nil {
		log.Printf("no se pudieron contar los votos: %v", err)
		writeError(w, http.StatusInternalServerError, "No se pudieron calcular los resultados")
		return
	}

	// Se parte de la lista de candidatos para que aparezcan los que tienen 0 votos.
	response := resultsResponse{ElectionID: electionID, Results: make([]candidateResult, 0, len(election.Candidates))}
	for _, c := range election.Candidates {
		response.TotalVotes += counts[c.ID]
		response.Results = append(response.Results, candidateResult{CandidateID: c.ID, CandidateName: c.Name, Votes: counts[c.ID]})
	}
	for i := range response.Results {
		if response.TotalVotes > 0 {
			pct := float64(response.Results[i].Votes) * 100 / float64(response.TotalVotes)
			response.Results[i].Percentage = math.Round(pct*100) / 100
		}
	}
	sort.SliceStable(response.Results, func(i, j int) bool {
		return response.Results[i].Votes > response.Results[j].Votes
	})
	writeJSON(w, http.StatusOK, response)
}

// fetchElection escribe la respuesta de error (404 o 502) y devuelve false si no la obtuvo.
func (a *App) fetchElection(w http.ResponseWriter, r *http.Request, id int) (*Election, bool) {
	election, err := a.clients.GetElection(r.Context(), id)
	if errors.Is(err, errElectionNotFound) {
		writeError(w, http.StatusNotFound, "La elección no existe")
		return nil, false
	}
	if err != nil {
		log.Printf("election service no disponible: %v", err)
		writeError(w, http.StatusBadGateway, "El Election Service no responde")
		return nil, false
	}
	return election, true
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
