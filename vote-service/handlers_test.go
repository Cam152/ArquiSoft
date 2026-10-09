package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeStore reemplaza a MongoDB en las pruebas.
type fakeStore struct {
	votes      []Vote
	audit      []AuditEvent
	failInsert bool
}

func (s *fakeStore) InsertVote(_ context.Context, vote Vote) error {
	if s.failInsert {
		return errors.New("mongo caído")
	}
	s.votes = append(s.votes, vote)
	return nil
}

func (s *fakeStore) InsertAudit(_ context.Context, event AuditEvent) error {
	s.audit = append(s.audit, event)
	return nil
}

func (s *fakeStore) CountByCandidate(_ context.Context, electionID int) (map[int]int, error) {
	counts := map[int]int{}
	for _, v := range s.votes {
		if v.ElectionID == electionID {
			counts[v.CandidateID]++
		}
	}
	return counts, nil
}

// fakeVoters simula el Voter Service: un solo votante con el token "good".
type fakeVoters struct {
	voted   bool
	marks   int
	unmarks int
}

func (f *fakeVoters) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /voters/mark-voted", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer good" || r.Header.Get("X-Service-Key") != "test-key" {
			writeError(w, http.StatusUnauthorized, "Token inválido")
			return
		}
		if f.voted {
			writeError(w, http.StatusConflict, "El votante ya votó")
			return
		}
		f.voted = true
		f.marks++
		writeJSON(w, http.StatusOK, map[string]string{"status": "marked"})
	})
	mux.HandleFunc("POST /voters/unmark-voted", func(w http.ResponseWriter, r *http.Request) {
		f.voted = false
		f.unmarks++
		writeJSON(w, http.StatusOK, map[string]string{"status": "unmarked"})
	})
	return mux
}

const electionsJSON = `{
  "1": {"id": 1, "is_open": true, "candidates": [{"id": 1, "name": "Ana Torres"}, {"id": 2, "name": "Luis Herrera"}, {"id": 3, "name": "Camila Duarte"}]},
  "2": {"id": 2, "is_open": false, "candidates": [{"id": 4, "name": "Otro"}]}
}`

type env struct {
	app    http.Handler
	store  *fakeStore
	voters *fakeVoters
}

func newEnv(t *testing.T) *env {
	t.Helper()
	var elections map[string]json.RawMessage
	if err := json.Unmarshal([]byte(electionsJSON), &elections); err != nil {
		t.Fatal(err)
	}
	electionSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, ok := elections[strings.TrimPrefix(r.URL.Path, "/elections/")]
		if !ok {
			writeError(w, http.StatusNotFound, "Elección no encontrada")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(raw)
	}))
	t.Cleanup(electionSrv.Close)

	voters := &fakeVoters{}
	voterSrv := httptest.NewServer(voters.handler())
	t.Cleanup(voterSrv.Close)

	cfg := Config{
		ElectionServiceURL: electionSrv.URL,
		VoterServiceURL:    voterSrv.URL,
		ServiceAPIKey:      "test-key",
		CORSOrigins:        []string{"http://localhost:3000"},
	}
	store := &fakeStore{}
	app := &App{cfg: cfg, store: store, clients: newClients(cfg)}
	return &env{app: app.routes(), store: store, voters: voters}
}

func (e *env) do(method, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.app.ServeHTTP(rec, req)
	return rec
}

func wantStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("HTTP %d, se esperaba %d (cuerpo: %s)", rec.Code, want, rec.Body.String())
	}
}

func TestHealth(t *testing.T) {
	e := newEnv(t)
	rec := e.do("GET", "/health", "", "")
	wantStatus(t, rec, http.StatusOK)
	if strings.TrimSpace(rec.Body.String()) != `{"status":"ok"}` {
		t.Fatalf("cuerpo inesperado: %s", rec.Body.String())
	}
}

func TestVoteIsRecordedAnonymously(t *testing.T) {
	e := newEnv(t)
	rec := e.do("POST", "/votes", "good", `{"election_id": 1, "candidate_id": 2}`)
	wantStatus(t, rec, http.StatusCreated)

	if len(e.store.votes) != 1 || len(e.store.audit) != 1 {
		t.Fatalf("se esperaba 1 voto y 1 evento, hay %d y %d", len(e.store.votes), len(e.store.audit))
	}
	vote := e.store.votes[0]
	if vote.ElectionID != 1 || vote.CandidateID != 2 || vote.CandidateName != "Luis Herrera" {
		t.Fatalf("voto inesperado: %+v", vote)
	}
	if !vote.CastAt.Equal(vote.CastAt.Truncate(time.Hour)) {
		t.Fatalf("cast_at no está redondeado a la hora: %v", vote.CastAt)
	}
}

func TestSecondVoteIsRejected(t *testing.T) {
	e := newEnv(t)
	wantStatus(t, e.do("POST", "/votes", "good", `{"election_id": 1, "candidate_id": 2}`), http.StatusCreated)
	wantStatus(t, e.do("POST", "/votes", "good", `{"election_id": 1, "candidate_id": 1}`), http.StatusConflict)
	if len(e.store.votes) != 1 {
		t.Fatalf("se guardaron %d votos", len(e.store.votes))
	}
}

func TestVoteRejections(t *testing.T) {
	cases := []struct {
		name, token, body string
		want              int
	}{
		{"cuerpo inválido", "good", `{"election_id": "1"}`, http.StatusUnprocessableEntity},
		{"falta candidate_id", "good", `{"election_id": 1}`, http.StatusUnprocessableEntity},
		{"sin token", "", `{"election_id": 1, "candidate_id": 2}`, http.StatusUnauthorized},
		{"token inválido", "bad", `{"election_id": 1, "candidate_id": 2}`, http.StatusUnauthorized},
		{"elección inexistente", "good", `{"election_id": 99, "candidate_id": 2}`, http.StatusNotFound},
		{"elección cerrada", "good", `{"election_id": 2, "candidate_id": 4}`, http.StatusConflict},
		{"candidato de otra elección", "good", `{"election_id": 1, "candidate_id": 4}`, http.StatusUnprocessableEntity},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			wantStatus(t, e.do("POST", "/votes", tc.token, tc.body), tc.want)
			if e.voters.voted || len(e.store.votes) != 0 {
				t.Fatalf("no debía quedar marcado ni guardarse el voto")
			}
		})
	}
}

func TestFailedInsertIsCompensated(t *testing.T) {
	e := newEnv(t)
	e.store.failInsert = true
	wantStatus(t, e.do("POST", "/votes", "good", `{"election_id": 1, "candidate_id": 2}`), http.StatusInternalServerError)
	if e.voters.marks != 1 || e.voters.unmarks != 1 || e.voters.voted {
		t.Fatalf("compensación incorrecta: %+v", e.voters)
	}
}

func TestUnreachableServicesReturn502(t *testing.T) {
	e := newEnv(t)
	cfg := Config{ElectionServiceURL: "http://127.0.0.1:1", VoterServiceURL: "http://127.0.0.1:1"}
	down := (&App{cfg: cfg, store: e.store, clients: newClients(cfg)}).routes()

	req := httptest.NewRequest("POST", "/votes", strings.NewReader(`{"election_id": 1, "candidate_id": 2}`))
	req.Header.Set("Authorization", "Bearer good")
	rec := httptest.NewRecorder()
	down.ServeHTTP(rec, req)
	wantStatus(t, rec, http.StatusBadGateway)
}

func TestResults(t *testing.T) {
	e := newEnv(t)
	for _, candidateID := range []int{2, 2, 1} {
		e.store.votes = append(e.store.votes, Vote{ElectionID: 1, CandidateID: candidateID})
	}
	rec := e.do("GET", "/results/1", "", "")
	wantStatus(t, rec, http.StatusOK)

	var got resultsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want := []candidateResult{
		{CandidateID: 2, CandidateName: "Luis Herrera", Votes: 2, Percentage: 66.67},
		{CandidateID: 1, CandidateName: "Ana Torres", Votes: 1, Percentage: 33.33},
		{CandidateID: 3, CandidateName: "Camila Duarte", Votes: 0, Percentage: 0},
	}
	if got.ElectionID != 1 || got.TotalVotes != 3 || len(got.Results) != len(want) {
		t.Fatalf("respuesta inesperada: %+v", got)
	}
	for i := range want {
		if got.Results[i] != want[i] {
			t.Fatalf("posición %d: %+v, se esperaba %+v", i, got.Results[i], want[i])
		}
	}

	wantStatus(t, e.do("GET", "/results/99", "", ""), http.StatusNotFound)
	wantStatus(t, e.do("GET", "/results/abc", "", ""), http.StatusUnprocessableEntity)
}

func TestCORSPreflight(t *testing.T) {
	e := newEnv(t)
	req := httptest.NewRequest("OPTIONS", "/votes", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()
	e.app.ServeHTTP(rec, req)
	wantStatus(t, rec, http.StatusNoContent)
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" ||
		!strings.Contains(rec.Header().Get("Access-Control-Allow-Headers"), "Authorization") {
		t.Fatalf("headers CORS inesperados: %v", rec.Header())
	}
}
