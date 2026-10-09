//go:build e2e

// Pruebas de punta a punta. Corren contra los servicios reales levantados con
// Docker Compose (ver run.sh): Vote Service, Votes DB, Election Service y
// Elections DB. Solo el Voter Service es simulado (mock-voter).
package e2e

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	voteURL     = getenv("VOTE_SERVICE_URL", "http://localhost:8002")
	electionURL = getenv("ELECTION_SERVICE_URL", "http://localhost:8000")
	mongoURI    = getenv("MONGO_URI", "mongodb://localhost:27017")
	mongoDB     = getenv("MONGO_DB", "votes")
	adminKey    = getenv("ADMIN_API_KEY", "dev-admin-key")

	client = &http.Client{Timeout: 10 * time.Second}
)

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ---------- Ayudas ----------

type response struct {
	status int
	header http.Header
	body   []byte
}

func (r response) detail() string {
	var body struct {
		Detail string `json:"detail"`
	}
	json.Unmarshal(r.body, &body)
	return body.Detail
}

func call(t *testing.T, method, url string, headers map[string]string, body any) response {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response{status: res.StatusCode, header: res.Header, body: raw}
}

func (r response) want(t *testing.T, status int) response {
	t.Helper()
	if r.status != status {
		t.Fatalf("HTTP %d, se esperaba %d (cuerpo: %s)", r.status, status, r.body)
	}
	return r
}

type election struct {
	ID         int
	Candidates []int // ids, en el orden en que se crearon
}

// newElection crea una elección en borrador con candidatos, usando la API de
// administración del Election Service.
func newElection(t *testing.T, candidates ...string) election {
	t.Helper()
	admin := map[string]string{"X-API-Key": adminKey}
	var created struct {
		ID int `json:"id"`
	}
	res := call(t, "POST", electionURL+"/elections", admin, map[string]string{"name": "E2E " + randomID()}).want(t, 201)
	if err := json.Unmarshal(res.body, &created); err != nil {
		t.Fatal(err)
	}
	e := election{ID: created.ID}
	for _, name := range candidates {
		var candidate struct {
			ID int `json:"id"`
		}
		url := fmt.Sprintf("%s/elections/%d/candidates", electionURL, e.ID)
		res := call(t, "POST", url, admin, map[string]string{"name": name}).want(t, 201)
		if err := json.Unmarshal(res.body, &candidate); err != nil {
			t.Fatal(err)
		}
		e.Candidates = append(e.Candidates, candidate.ID)
	}
	return e
}

func (e election) transition(t *testing.T, action string) {
	t.Helper()
	url := fmt.Sprintf("%s/elections/%d/%s", electionURL, e.ID, action)
	call(t, "POST", url, map[string]string{"X-API-Key": adminKey}, nil).want(t, 200)
}

func openElection(t *testing.T, candidates ...string) election {
	t.Helper()
	e := newElection(t, candidates...)
	e.transition(t, "activate")
	return e
}

// newVoter devuelve un token nuevo: para mock-voter cada token es un votante distinto.
func newVoter() string { return "voter-" + randomID() }

func randomID() string {
	raw := make([]byte, 6)
	rand.Read(raw)
	return hex.EncodeToString(raw)
}

func vote(t *testing.T, token string, electionID, candidateID int) response {
	t.Helper()
	headers := map[string]string{}
	if token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	return call(t, "POST", voteURL+"/votes", headers, map[string]int{"election_id": electionID, "candidate_id": candidateID})
}

type results struct {
	ElectionID int `json:"election_id"`
	TotalVotes int `json:"total_votes"`
	Results    []struct {
		CandidateID   int     `json:"candidate_id"`
		CandidateName string  `json:"candidate_name"`
		Votes         int     `json:"votes"`
		Percentage    float64 `json:"percentage"`
	} `json:"results"`
}

func getResults(t *testing.T, electionID int) results {
	t.Helper()
	res := call(t, "GET", fmt.Sprintf("%s/results/%d", voteURL, electionID), nil, nil).want(t, 200)
	var out results
	if err := json.Unmarshal(res.body, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func votesDB(t *testing.T) *mongo.Database {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Disconnect(context.Background()) })
	return conn.Database(mongoDB)
}

// ---------- Pruebas ----------

func TestServicesAreHealthy(t *testing.T) {
	for name, url := range map[string]string{"vote-service": voteURL, "election-service": electionURL} {
		res := call(t, "GET", url+"/health", nil, nil).want(t, 200)
		if !strings.Contains(string(res.body), `"ok"`) {
			t.Fatalf("%s: cuerpo inesperado: %s", name, res.body)
		}
	}
}

// El flujo completo de la sección 7 del README: votar, no poder repetir y ver el conteo.
func TestVoteFlowAndResults(t *testing.T) {
	e := openElection(t, "Ana", "Luis", "Camila")
	ana, luis, camila := e.Candidates[0], e.Candidates[1], e.Candidates[2]

	empty := getResults(t, e.ID)
	if empty.TotalVotes != 0 || len(empty.Results) != 3 {
		t.Fatalf("una elección sin votos debe listar sus 3 candidatos en 0: %+v", empty)
	}

	first := newVoter()
	res := vote(t, first, e.ID, luis).want(t, 201)
	if !strings.Contains(string(res.body), `"recorded"`) {
		t.Fatalf("cuerpo inesperado: %s", res.body)
	}

	// El mismo votante no puede votar dos veces, ni siquiera por otro candidato.
	vote(t, first, e.ID, ana).want(t, 409)

	vote(t, newVoter(), e.ID, luis).want(t, 201)
	vote(t, newVoter(), e.ID, ana).want(t, 201)

	got := getResults(t, e.ID)
	if got.ElectionID != e.ID || got.TotalVotes != 3 {
		t.Fatalf("totales inesperados: %+v", got)
	}
	want := []struct {
		id         int
		name       string
		votes      int
		percentage float64
	}{{luis, "Luis", 2, 66.67}, {ana, "Ana", 1, 33.33}, {camila, "Camila", 0, 0}}
	for i, w := range want {
		r := got.Results[i]
		if r.CandidateID != w.id || r.CandidateName != w.name || r.Votes != w.votes || r.Percentage != w.percentage {
			t.Fatalf("posición %d: %+v, se esperaba %+v", i, r, w)
		}
	}
}

func TestVoteRequiresValidToken(t *testing.T) {
	e := openElection(t, "Ana", "Luis")
	vote(t, "", e.ID, e.Candidates[0]).want(t, 401)
	vote(t, "invalid", e.ID, e.Candidates[0]).want(t, 401)
	if got := getResults(t, e.ID); got.TotalVotes != 0 {
		t.Fatalf("se guardaron %d votos sin token válido", got.TotalVotes)
	}
}

// La elección se valida antes de marcar al votante: un intento en una elección
// que no está abierta no le gasta el voto.
func TestClosedElectionDoesNotConsumeTheVote(t *testing.T) {
	draft := newElection(t, "Ana", "Luis")
	closed := openElection(t, "Ana", "Luis")
	closed.transition(t, "close")
	open := openElection(t, "Ana", "Luis")

	voter := newVoter()
	vote(t, voter, draft.ID, draft.Candidates[0]).want(t, 409)
	vote(t, voter, closed.ID, closed.Candidates[0]).want(t, 409)
	if got := getResults(t, closed.ID); got.TotalVotes != 0 {
		t.Fatalf("se guardaron %d votos en una elección cerrada", got.TotalVotes)
	}
	vote(t, voter, open.ID, open.Candidates[0]).want(t, 201)
}

func TestInvalidVotesAreRejected(t *testing.T) {
	e := openElection(t, "Ana", "Luis")
	other := openElection(t, "Pedro", "Marta")
	voter := newVoter()

	vote(t, voter, 99999999, e.Candidates[0]).want(t, 404)
	vote(t, voter, e.ID, other.Candidates[0]).want(t, 422)

	auth := map[string]string{"Authorization": "Bearer " + voter}
	call(t, "POST", voteURL+"/votes", auth, map[string]string{"election_id": "uno"}).want(t, 422)
	call(t, "POST", voteURL+"/votes", auth, map[string]int{"election_id": e.ID}).want(t, 422)

	call(t, "GET", voteURL+"/results/99999999", nil, nil).want(t, 404)
	call(t, "GET", voteURL+"/results/abc", nil, nil).want(t, 422)

	// Ningún rechazo anterior marcó al votante.
	vote(t, voter, e.ID, e.Candidates[0]).want(t, 201)
}

func TestErrorsUseDetailFormat(t *testing.T) {
	res := call(t, "GET", voteURL+"/results/99999999", nil, nil).want(t, 404)
	if res.detail() == "" || !strings.HasPrefix(res.header.Get("Content-Type"), "application/json") {
		t.Fatalf(`se esperaba JSON {"detail": "..."}: %s`, res.body)
	}
}

// El voto guardado en MongoDB no lleva nada que identifique al votante.
func TestStoredVoteIsAnonymous(t *testing.T) {
	e := openElection(t, "Ana", "Luis")
	voter := newVoter()
	vote(t, voter, e.ID, e.Candidates[1]).want(t, 201)

	db := votesDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var stored []bson.M
	cursor, err := db.Collection("votes").Find(ctx, bson.M{"election_id": e.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := cursor.All(ctx, &stored); err != nil {
		t.Fatal(err)
	}
	if len(stored) != 1 {
		t.Fatalf("se esperaba 1 voto en MongoDB, hay %d", len(stored))
	}
	doc := stored[0]

	var fields []string
	for field := range doc {
		fields = append(fields, field)
	}
	slices.Sort(fields)
	if want := []string{"_id", "candidate_id", "candidate_name", "cast_at", "election_id"}; !slices.Equal(fields, want) {
		t.Fatalf("campos del voto: %v, se esperaba %v", fields, want)
	}
	if raw, _ := bson.MarshalExtJSON(doc, false, false); strings.Contains(string(raw), voter) {
		t.Fatalf("el voto guardado contiene el token del votante: %s", raw)
	}
	if doc["candidate_name"] != "Luis" {
		t.Fatalf("candidate_name inesperado: %v", doc["candidate_name"])
	}
	castAt := doc["cast_at"].(primitive.DateTime).Time()
	if !castAt.Equal(castAt.Truncate(time.Hour)) {
		t.Fatalf("cast_at no está redondeado a la hora: %v", castAt)
	}

	events, err := db.Collection("audit").CountDocuments(ctx, bson.M{"election_id": e.ID, "type": "vote_registered"})
	if err != nil {
		t.Fatal(err)
	}
	if events != 1 {
		t.Fatalf("se esperaba 1 evento de auditoría, hay %d", events)
	}
}

func TestCORSPreflight(t *testing.T) {
	headers := map[string]string{"Origin": "http://localhost:3000", "Access-Control-Request-Method": "POST"}
	res := call(t, "OPTIONS", voteURL+"/votes", headers, nil).want(t, 204)
	if res.header.Get("Access-Control-Allow-Origin") != "http://localhost:3000" ||
		!strings.Contains(res.header.Get("Access-Control-Allow-Headers"), "Authorization") {
		t.Fatalf("headers CORS inesperados: %v", res.header)
	}

	headers["Origin"] = "http://otro-sitio.example"
	res = call(t, "OPTIONS", voteURL+"/votes", headers, nil)
	if got := res.header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("no debía permitir el origen, devolvió %q", got)
	}
}
