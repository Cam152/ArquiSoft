package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"vote-service/internal/handler"
	"vote-service/internal/model"
	"vote-service/internal/service"
)

// Estas pruebas recorren handler y service reales. Solo se simulan MongoDB y el
// outbox.

type memRepo struct {
	mu        sync.Mutex
	byKey     map[string]model.Vote
	insertErr error
	pingErr   error
}

func (r *memRepo) Insert(_ context.Context, v model.Vote) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.insertErr != nil {
		return r.insertErr
	}
	if _, ok := r.byKey[v.IdempotencyKey]; ok {
		return model.ErrDuplicateKey
	}
	r.byKey[v.IdempotencyKey] = v
	return nil
}

func (r *memRepo) FindByIdempotencyKey(_ context.Context, key string) (*model.Vote, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if v, ok := r.byKey[key]; ok {
		return &v, nil
	}
	return nil, nil
}

func (r *memRepo) Ping(context.Context) error { return r.pingErr }

type countingOutbox struct{ n int }

func (o *countingOutbox) Dispatch(model.Vote) { o.n++ }

type env struct {
	app    http.Handler
	repo   *memRepo
	outbox *countingOutbox
}

func newEnv() *env {
	repo := &memRepo{byKey: map[string]model.Vote{}}
	outbox := &countingOutbox{}
	app := handler.New(service.NewVotes(repo, outbox), repo, "test-key").Routes()
	return &env{app: app, repo: repo, outbox: outbox}
}

const goodBody = `{"election_id": 1, "candidate_id": 2, "candidate_name": "Luis Herrera"}`

func (e *env) post(body string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/votes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	e.app.ServeHTTP(rec, req)
	return rec
}

func authed(key string) map[string]string {
	return map[string]string{"X-Service-Key": "test-key", "Idempotency-Key": key}
}

func wantStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("HTTP %d, se esperaba %d (cuerpo: %s)", rec.Code, want, rec.Body.String())
	}
}

func wantDetail(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	var body struct {
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Detail == "" ||
		!strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf(`se esperaba JSON {"detail": "..."}: %s`, rec.Body.String())
	}
}

func TestHealth(t *testing.T) {
	e := newEnv()
	rec := httptest.NewRecorder()
	e.app.ServeHTTP(rec, httptest.NewRequest("GET", "/health", nil))
	wantStatus(t, rec, http.StatusOK)
	if strings.TrimSpace(rec.Body.String()) != `{"status":"ok"}` {
		t.Fatalf("cuerpo inesperado: %s", rec.Body.String())
	}

	e.repo.pingErr = errors.New("mongo caído")
	rec = httptest.NewRecorder()
	e.app.ServeHTTP(rec, httptest.NewRequest("GET", "/health", nil))
	wantStatus(t, rec, http.StatusServiceUnavailable)
	wantDetail(t, rec)
}

func TestVoteIsRecorded(t *testing.T) {
	e := newEnv()
	rec := e.post(goodBody, authed("key-1"))
	wantStatus(t, rec, http.StatusCreated)
	if strings.TrimSpace(rec.Body.String()) != `{"status":"recorded"}` {
		t.Fatalf("cuerpo inesperado: %s", rec.Body.String())
	}
	v := e.repo.byKey["key-1"]
	if v.ElectionID != 1 || v.CandidateID != 2 || v.CandidateName != "Luis Herrera" || v.Published {
		t.Fatalf("voto inesperado: %+v", v)
	}
	if e.outbox.n != 1 {
		t.Fatalf("se esperaba despachar 1 evento, se despacharon %d", e.outbox.n)
	}
}

// El reintento del gateway con la misma llave responde 201 sin duplicar.
func TestRetryWithSameKey(t *testing.T) {
	e := newEnv()
	wantStatus(t, e.post(goodBody, authed("key-1")), http.StatusCreated)
	wantStatus(t, e.post(goodBody, authed("key-1")), http.StatusCreated)
	if len(e.repo.byKey) != 1 || e.outbox.n != 1 {
		t.Fatalf("votos %d, eventos %d; se esperaba 1 y 1", len(e.repo.byKey), e.outbox.n)
	}

	other := `{"election_id": 1, "candidate_id": 3, "candidate_name": "Camila Duarte"}`
	rec := e.post(other, authed("key-1"))
	wantStatus(t, rec, http.StatusConflict)
	wantDetail(t, rec)
}

func TestRejections(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		headers map[string]string
		want    int
	}{
		{"sin llave de servicio", goodBody, map[string]string{"Idempotency-Key": "k"}, http.StatusUnauthorized},
		{"llave de servicio incorrecta", goodBody, map[string]string{"X-Service-Key": "otra", "Idempotency-Key": "k"}, http.StatusUnauthorized},
		{"sin Idempotency-Key", goodBody, map[string]string{"X-Service-Key": "test-key"}, http.StatusUnprocessableEntity},
		{"JSON inválido", `{"election_id":`, authed("k"), http.StatusUnprocessableEntity},
		{"tipo incorrecto", `{"election_id": "1", "candidate_id": 2, "candidate_name": "A"}`, authed("k"), http.StatusUnprocessableEntity},
		{"falta candidate_name", `{"election_id": 1, "candidate_id": 2}`, authed("k"), http.StatusUnprocessableEntity},
		{"ids no positivos", `{"election_id": 0, "candidate_id": 2, "candidate_name": "A"}`, authed("k"), http.StatusUnprocessableEntity},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv()
			rec := e.post(tc.body, tc.headers)
			wantStatus(t, rec, tc.want)
			wantDetail(t, rec)
			if len(e.repo.byKey) != 0 || e.outbox.n != 0 {
				t.Fatal("no debía guardarse ni publicarse el voto")
			}
		})
	}
}

func TestStorageFailureReturns500(t *testing.T) {
	e := newEnv()
	e.repo.insertErr = errors.New("mongo caído")
	rec := e.post(goodBody, authed("k"))
	wantStatus(t, rec, http.StatusInternalServerError)
	wantDetail(t, rec)
	if e.outbox.n != 0 {
		t.Fatal("no debe publicarse un voto que no se guardó")
	}
}

// Las rutas del P1 ya no existen: los resultados están en agr-results-lo.
func TestUnknownRoutes(t *testing.T) {
	e := newEnv()
	for _, path := range []string{"/results/1", "/nada"} {
		rec := httptest.NewRecorder()
		e.app.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		wantStatus(t, rec, http.StatusNotFound)
		wantDetail(t, rec)
	}
}
