package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeGateway responde con respuestas fijas por "MÉTODO /ruta" y anota cada
// petición recibida.
type fakeGateway struct {
	t         *testing.T
	responses map[string]reply
	requests  []recorded
}

type reply struct {
	status int
	body   string
}

type recorded struct {
	method, path, query, apiKey, contentType string
	body                                     map[string]any
}

func newGateway(t *testing.T, responses map[string]reply) (*fakeGateway, string) {
	g := &fakeGateway{t: t, responses: responses}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := recorded{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery,
			apiKey: r.Header.Get("X-API-Key"), contentType: r.Header.Get("Content-Type")}
		if raw, _ := io.ReadAll(r.Body); len(raw) > 0 {
			if err := json.Unmarshal(raw, &rec.body); err != nil {
				t.Errorf("cuerpo no es JSON: %s", raw)
			}
		}
		g.requests = append(g.requests, rec)
		resp, ok := responses[r.Method+" "+r.URL.Path]
		if !ok {
			resp = reply{404, `{"detail":"Recurso no encontrado"}`}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.status)
		io.WriteString(w, resp.body)
	}))
	t.Cleanup(srv.Close)
	return g, srv.URL
}

func (g *fakeGateway) only() recorded {
	g.t.Helper()
	if len(g.requests) != 1 {
		g.t.Fatalf("se esperaba 1 petición, hubo %d: %+v", len(g.requests), g.requests)
	}
	return g.requests[0]
}

type result struct {
	code           int
	stdout, stderr string
}

// run ejecuta la CLI contra el gateway falso, con la llave "k" y stdin dado.
func run(t *testing.T, gatewayURL, stdin string, args ...string) result {
	t.Helper()
	t.Setenv("AGORA_GATEWAY_URL", gatewayURL)
	t.Setenv("AGORA_ADMIN_KEY", "k")
	var out, errOut bytes.Buffer
	root := NewRoot()
	root.SetArgs(args)
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetIn(strings.NewReader(stdin))
	code := 0
	if err := root.ExecuteContext(context.Background()); err != nil {
		errOut.WriteString("Error: " + err.Error() + "\n")
		code = 1
	}
	return result{code, out.String(), errOut.String()}
}

const electionJSON = `{"id": 4, "name": "Elección 2026", "description": "Prueba", "status": "draft",
  "starts_at": null, "ends_at": null, "created_at": "2026-11-01T10:00:00Z", "is_open": false,
  "candidate_count": 2, "candidates": [
    {"id": 10, "election_id": 4, "number": 1, "name": "Ana Torres", "description": null},
    {"id": 11, "election_id": 4, "number": 2, "name": "Luis Herrera", "description": null}]}`

func TestElectionsList(t *testing.T) {
	g, url := newGateway(t, map[string]reply{
		"GET /elections": {200, `[{"id": 1, "name": "Representante 2026", "status": "active", "is_open": true, "candidate_count": 3, "created_at": "2026-11-01T10:00:00Z"}]`},
	})
	r := run(t, url, "", "elections", "list", "--status", "active")
	if r.code != 0 {
		t.Fatalf("código %d: %s", r.code, r.stderr)
	}
	req := g.only()
	if req.method != "GET" || req.query != "status=active" || req.apiKey != "" {
		t.Fatalf("petición inesperada: %+v (la llave no debe ir a rutas públicas)", req)
	}
	for _, want := range []string{"ID", "NOMBRE", "Representante 2026", "activa", "sí", "3"} {
		if !strings.Contains(r.stdout, want) {
			t.Fatalf("falta %q en la salida:\n%s", want, r.stdout)
		}
	}
}

func TestElectionsListRejectsUnknownStatus(t *testing.T) {
	g, url := newGateway(t, nil)
	if r := run(t, url, "", "elections", "list", "--status", "open"); r.code == 0 || len(g.requests) != 0 {
		t.Fatalf("debía fallar sin llamar al gateway: %+v", r)
	}
}

func TestElectionsShow(t *testing.T) {
	_, url := newGateway(t, map[string]reply{"GET /elections/4": {200, electionJSON}})
	r := run(t, url, "", "elections", "show", "4")
	for _, want := range []string{"Elección 2026", "Prueba", "borrador", "Ana Torres", "Luis Herrera"} {
		if !strings.Contains(r.stdout, want) {
			t.Fatalf("falta %q en la salida:\n%s", want, r.stdout)
		}
	}
}

func TestElectionsCreate(t *testing.T) {
	g, url := newGateway(t, map[string]reply{"POST /admin/elections": {201, electionJSON}})
	r := run(t, url, "", "elections", "create", "--name", "Elección 2026", "--description", "Prueba",
		"--starts", "2026-11-03T08:00:00-05:00")
	if r.code != 0 {
		t.Fatalf("código %d: %s", r.code, r.stderr)
	}
	req := g.only()
	if req.apiKey != "k" || req.contentType != "application/json" {
		t.Fatalf("faltan headers: %+v", req)
	}
	if req.body["name"] != "Elección 2026" || req.body["description"] != "Prueba" ||
		req.body["starts_at"] != "2026-11-03T08:00:00-05:00" {
		t.Fatalf("cuerpo inesperado: %v", req.body)
	}
	if _, sent := req.body["ends_at"]; sent {
		t.Fatalf("no debía enviar ends_at: %v", req.body)
	}
	if !strings.Contains(r.stdout, "Elección 4 creada") {
		t.Fatalf("salida inesperada: %s", r.stdout)
	}
}

func TestElectionsCreateValidatesBeforeCalling(t *testing.T) {
	g, url := newGateway(t, nil)
	for _, args := range [][]string{
		{"elections", "create"}, // falta --name
		{"elections", "create", "--name", "X", "--starts", "mañana"},
	} {
		if r := run(t, url, "", args...); r.code == 0 {
			t.Fatalf("%v debía fallar", args)
		}
	}
	if len(g.requests) != 0 {
		t.Fatalf("no debía llamar al gateway: %+v", g.requests)
	}
}

func TestElectionsUpdateSendsOnlyChangedFields(t *testing.T) {
	g, url := newGateway(t, map[string]reply{"PATCH /admin/elections/4": {200, electionJSON}})
	r := run(t, url, "", "elections", "update", "4", "--description", "")
	if r.code != 0 {
		t.Fatalf("código %d: %s", r.code, r.stderr)
	}
	body := g.only().body
	if len(body) != 1 || body["description"] != "" {
		t.Fatalf("solo debía enviar description vacía: %v", body)
	}

	if r := run(t, url, "", "elections", "update", "4"); r.code == 0 || !strings.Contains(r.stderr, "al menos un campo") {
		t.Fatalf("sin campos debía fallar: %+v", r)
	}
}

func TestElectionsOpenAndClose(t *testing.T) {
	g, url := newGateway(t, map[string]reply{
		"POST /admin/elections/4/activate": {200, strings.Replace(electionJSON, `"draft"`, `"active"`, 1)},
		"POST /admin/elections/4/close":    {200, strings.Replace(electionJSON, `"draft"`, `"closed"`, 1)},
	})
	if r := run(t, url, "", "elections", "open", "4"); r.code != 0 || !strings.Contains(r.stdout, "activa") {
		t.Fatalf("open: %+v", r)
	}
	if r := run(t, url, "", "elections", "close", "4"); r.code != 0 || !strings.Contains(r.stdout, "cerrada") {
		t.Fatalf("close: %+v", r)
	}
	if len(g.requests) != 2 || g.requests[0].apiKey != "k" || g.requests[1].apiKey != "k" {
		t.Fatalf("peticiones inesperadas: %+v", g.requests)
	}
}

func TestElectionsDeleteAsksForConfirmation(t *testing.T) {
	g, url := newGateway(t, map[string]reply{"DELETE /admin/elections/4": {204, ""}})

	if r := run(t, url, "n\n", "elections", "delete", "4"); r.code != 0 || !strings.Contains(r.stdout, "Cancelado") {
		t.Fatalf("debía cancelar: %+v", r)
	}
	if len(g.requests) != 0 {
		t.Fatal("no debía borrar sin confirmación")
	}
	if r := run(t, url, "s\n", "elections", "delete", "4"); r.code != 0 || !strings.Contains(r.stdout, "borrada") {
		t.Fatalf("debía borrar: %+v", r)
	}
	if r := run(t, url, "", "elections", "delete", "4", "--yes"); r.code != 0 {
		t.Fatalf("--yes debía borrar sin preguntar: %+v", r)
	}
	if len(g.requests) != 2 || g.requests[0].method != "DELETE" {
		t.Fatalf("peticiones inesperadas: %+v", g.requests)
	}
}

func TestCandidates(t *testing.T) {
	g, url := newGateway(t, map[string]reply{
		"POST /admin/elections/4/candidates":      {201, `{"id": 12, "election_id": 4, "number": 3, "name": "Camila Duarte"}`},
		"DELETE /admin/elections/4/candidates/12": {204, ""},
	})
	r := run(t, url, "", "candidates", "add", "--election", "4", "--name", "  Camila Duarte ")
	if r.code != 0 || !strings.Contains(r.stdout, "Candidato 12") {
		t.Fatalf("add: %+v", r)
	}
	if body := g.requests[0].body; body["name"] != "Camila Duarte" || len(body) != 1 {
		t.Fatalf("cuerpo inesperado: %v", body)
	}
	if r := run(t, url, "", "candidates", "remove", "--election", "4", "--candidate", "12"); r.code != 0 {
		t.Fatalf("remove: %+v", r)
	}
	if r := run(t, url, "", "candidates", "remove", "--election", "4"); r.code == 0 {
		t.Fatal("remove sin --candidate debía fallar")
	}
}

func TestResultsShow(t *testing.T) {
	_, url := newGateway(t, map[string]reply{"GET /results/1": {200, `{"election_id": 1, "election_name": "Representante 2026",
		"total_votes": 10, "results": [{"candidate_id": 2, "candidate_name": "Luis Herrera", "votes": 6, "percentage": 60.0},
		{"candidate_id": 1, "candidate_name": "Ana Torres", "votes": 4, "percentage": 40.0}]}`}})
	r := run(t, url, "", "results", "show", "1")
	for _, want := range []string{"Representante 2026", "Total de votos: 10", "Luis Herrera", "60.00", "████████████"} {
		if !strings.Contains(r.stdout, want) {
			t.Fatalf("falta %q en la salida:\n%s", want, r.stdout)
		}
	}
}

func TestAuditList(t *testing.T) {
	g, url := newGateway(t, map[string]reply{"GET /admin/audit": {200,
		`[{"event_id": "abc", "type": "VoteRegistered", "election_id": 4, "occurred_at": "2026-11-03T14:00:00Z"}]`}})
	r := run(t, url, "", "audit", "list", "--election", "4", "--limit", "5")
	req := g.only()
	if req.query != "election_id=4&limit=5" || req.apiKey != "k" {
		t.Fatalf("petición inesperada: %+v", req)
	}
	if !strings.Contains(r.stdout, "VoteRegistered") || !strings.Contains(r.stdout, "abc") {
		t.Fatalf("salida inesperada:\n%s", r.stdout)
	}
}

func TestJSONOutput(t *testing.T) {
	_, url := newGateway(t, map[string]reply{"GET /elections/4": {200, electionJSON}})
	r := run(t, url, "", "--json", "elections", "show", "4")
	var e map[string]any
	if err := json.Unmarshal([]byte(r.stdout), &e); err != nil || e["id"] != 4.0 {
		t.Fatalf("salida no es JSON válido: %s", r.stdout)
	}
}

func TestErrorsShowDetailAndFail(t *testing.T) {
	_, url := newGateway(t, map[string]reply{
		"POST /admin/elections/4/activate": {409, `{"detail": "La elección necesita al menos 2 candidatos"}`},
		"POST /admin/elections":            {422, `{"detail": [{"loc": ["body", "name"], "msg": "String should have at least 3 characters"}]}`},
	})
	r := run(t, url, "", "elections", "open", "4")
	if r.code != 1 || !strings.Contains(r.stderr, "al menos 2 candidatos") || !strings.Contains(r.stderr, "409") {
		t.Fatalf("debía mostrar el detail y salir con 1: %+v", r)
	}
	r = run(t, url, "", "elections", "create", "--name", "X")
	if r.code != 1 || !strings.Contains(r.stderr, "name: String should have at least 3 characters") {
		t.Fatalf("debía resumir el 422 de FastAPI: %+v", r)
	}
	r = run(t, url, "", "elections", "show", "99")
	if r.code != 1 || !strings.Contains(r.stderr, "Recurso no encontrado") {
		t.Fatalf("404: %+v", r)
	}
}

func TestAdminCommandsNeedKey(t *testing.T) {
	g, url := newGateway(t, nil)
	t.Setenv("AGORA_GATEWAY_URL", url)
	t.Setenv("AGORA_ADMIN_KEY", "")
	var out, errOut bytes.Buffer
	code := Execute(context.Background(), []string{"elections", "open", "4"}, &out, &errOut)
	if code != 1 || !strings.Contains(errOut.String(), "AGORA_ADMIN_KEY") || len(g.requests) != 0 {
		t.Fatalf("debía fallar sin llamar al gateway: código %d, %s", code, errOut.String())
	}
}

func TestGatewayDown(t *testing.T) {
	r := run(t, "http://127.0.0.1:1", "", "elections", "list")
	if r.code != 1 || !strings.Contains(r.stderr, "no se pudo conectar con el gateway") {
		t.Fatalf("resultado inesperado: %+v", r)
	}
}

func TestInvalidIDs(t *testing.T) {
	g, url := newGateway(t, nil)
	for _, args := range [][]string{{"elections", "show", "abc"}, {"elections", "open", "0"}, {"results", "show", "-1"}} {
		if r := run(t, url, "", args...); r.code == 0 {
			t.Fatalf("%v debía fallar", args)
		}
	}
	if len(g.requests) != 0 {
		t.Fatal("no debía llamar al gateway con ids inválidos")
	}
}
