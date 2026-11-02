//go:build e2e

// Pruebas de punta a punta de agr-vote-lo. Corren contra los contenedores reales
// levantados con Docker Compose (ver run.sh): agr-vote-lo, agr-votes-db
// (MongoDB) y agr-broker-as (RabbitMQ con su definitions.json). Nada es simulado.
package e2e

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	voteURL    = getenv("VOTE_SERVICE_URL", "http://localhost:8002")
	mongoURI   = getenv("MONGO_URI", "mongodb://votes:votes@localhost:27017/?authSource=admin")
	mongoDB    = getenv("MONGO_DB", "votes")
	amqpURL    = getenv("AMQP_URL", "amqp://agora:agora@localhost:5672/")
	serviceKey = getenv("SERVICE_API_KEY", "dev-service-key")

	httpClient = &http.Client{Timeout: 10 * time.Second}
)

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ---------- Ayudas ----------

func randomKey() string {
	raw := make([]byte, 8)
	rand.Read(raw)
	return hex.EncodeToString(raw)
}

type response struct {
	status int
	body   []byte
}

func postVote(t *testing.T, headers map[string]string, body any) response {
	t.Helper()
	raw, _ := json.Marshal(body)
	req, err := http.NewRequest("POST", voteURL+"/votes", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := httpClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	out, _ := io.ReadAll(res.Body)
	return response{res.StatusCode, out}
}

func (r response) want(t *testing.T, status int) {
	t.Helper()
	if r.status != status {
		t.Fatalf("HTTP %d, se esperaba %d (cuerpo: %s)", r.status, status, r.body)
	}
}

func asGateway(key string) map[string]string {
	return map[string]string{"X-Service-Key": serviceKey, "Idempotency-Key": key}
}

func vote(electionID, candidateID int, name string) map[string]any {
	return map[string]any{"election_id": electionID, "candidate_id": candidateID, "candidate_name": name}
}

func votes(t *testing.T) *mongo.Collection {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Disconnect(context.Background()) })
	return conn.Database(mongoDB).Collection("votes")
}

func findByKey(t *testing.T, key string) []bson.M {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cursor, err := votes(t).Find(ctx, bson.M{"idempotency_key": key})
	if err != nil {
		t.Fatal(err)
	}
	var docs []bson.M
	if err := cursor.All(ctx, &docs); err != nil {
		t.Fatal(err)
	}
	return docs
}

func waitPublished(t *testing.T, id string) {
	t.Helper()
	col := votes(t)
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		var doc bson.M
		if err := col.FindOne(context.Background(), bson.M{"_id": id}).Decode(&doc); err == nil && doc["published"] == true {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("el voto %s no quedó published: true", id)
}

func broker(t *testing.T) *amqp.Channel {
	t.Helper()
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		t.Fatalf("no se pudo conectar a RabbitMQ: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	ch, err := conn.Channel()
	if err != nil {
		t.Fatal(err)
	}
	return ch
}

// listen enlaza una cola temporal al exchange agora.votes para ver los eventos
// sin quitárselos a las colas de resultados y auditoría.
func listen(t *testing.T) <-chan amqp.Delivery {
	t.Helper()
	ch := broker(t)
	q, err := ch.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := ch.QueueBind(q.Name, "", "agora.votes", false, nil); err != nil {
		t.Fatal(err)
	}
	deliveries, err := ch.Consume(q.Name, "", true, true, false, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	return deliveries
}

func waitEvent(t *testing.T, deliveries <-chan amqp.Delivery, eventID string) amqp.Delivery {
	t.Helper()
	timeout := time.After(20 * time.Second)
	for {
		select {
		case d := <-deliveries:
			if d.MessageId == eventID {
				return d
			}
		case <-timeout:
			t.Fatalf("no llegó el evento %s al exchange agora.votes", eventID)
		}
	}
}

func queueDepth(t *testing.T, name string) int {
	t.Helper()
	q, err := broker(t).QueueDeclarePassive(name, true, false, false, false, nil)
	if err != nil {
		t.Fatalf("la cola %s no existe: %v", name, err)
	}
	return q.Messages
}

// ---------- Pruebas ----------

func TestHealth(t *testing.T) {
	res, err := httpClient.Get(voteURL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 || !strings.Contains(string(body), `"ok"`) {
		t.Fatalf("health: HTTP %d %s", res.StatusCode, body)
	}
}

// La topología de agr-broker-as (definitions.json) quedó cargada.
func TestBrokerTopology(t *testing.T) {
	ch := broker(t)
	if err := ch.ExchangeDeclarePassive("agora.votes", "fanout", true, false, false, false, nil); err != nil {
		t.Fatalf("exchange agora.votes: %v", err)
	}
	ch = broker(t)
	if err := ch.ExchangeDeclarePassive("agora.dlx", "direct", true, false, false, false, nil); err != nil {
		t.Fatalf("exchange agora.dlx: %v", err)
	}
	for _, q := range []string{"results.vote-registered", "audit.vote-registered", "results.dead", "audit.dead"} {
		queueDepth(t, q)
	}
}

// Flujo completo: el voto se guarda anónimo, sale el evento con el contrato de
// la sección 5.3 y llega a las colas de resultados y auditoría.
func TestVoteIsStoredAndPublished(t *testing.T) {
	events := listen(t)
	resultsBefore := queueDepth(t, "results.vote-registered")
	auditBefore := queueDepth(t, "audit.vote-registered")

	key := randomKey()
	res := postVote(t, asGateway(key), vote(1, 2, "Luis Herrera"))
	res.want(t, 201)
	if strings.TrimSpace(string(res.body)) != `{"status":"recorded"}` {
		t.Fatalf("cuerpo inesperado: %s", res.body)
	}

	docs := findByKey(t, key)
	if len(docs) != 1 {
		t.Fatalf("se esperaba 1 voto en MongoDB, hay %d", len(docs))
	}
	doc := docs[0]
	var fields []string
	for f := range doc {
		fields = append(fields, f)
	}
	slices.Sort(fields)
	want := []string{"_id", "candidate_id", "candidate_name", "cast_at", "election_id", "idempotency_key", "published"}
	if !slices.Equal(fields, want) {
		t.Fatalf("campos del voto: %v, se esperaba %v (ningún dato del votante)", fields, want)
	}
	id, _ := doc["_id"].(string)
	if len(id) != 24 {
		t.Fatalf("_id debe ser un string aleatorio de 24 caracteres, es %v", doc["_id"])
	}

	d := waitEvent(t, events, id)
	if d.ContentType != "application/json" || d.DeliveryMode != amqp.Persistent || d.Type != "VoteRegistered" {
		t.Fatalf("propiedades del mensaje: %+v", d)
	}
	var event map[string]any
	if err := json.Unmarshal(d.Body, &event); err != nil {
		t.Fatal(err)
	}
	if event["event_id"] != id || event["type"] != "VoteRegistered" || event["version"] != 1.0 ||
		event["election_id"] != 1.0 || event["candidate_id"] != 2.0 {
		t.Fatalf("evento inesperado: %v", event)
	}
	occurred, err := time.Parse(time.RFC3339, event["occurred_at"].(string))
	if err != nil || !occurred.Equal(occurred.Truncate(time.Hour)) {
		t.Fatalf("occurred_at debe ir en RFC 3339 redondeado a la hora: %v", event["occurred_at"])
	}
	if _, leaked := event["candidate_name"]; leaked {
		t.Fatalf("el evento solo debe llevar lo necesario para contar: %v", event)
	}

	waitPublished(t, id)
	if got := queueDepth(t, "results.vote-registered"); got <= resultsBefore {
		t.Fatalf("results.vote-registered no recibió el evento (%d → %d)", resultsBefore, got)
	}
	if got := queueDepth(t, "audit.vote-registered"); got <= auditBefore {
		t.Fatalf("audit.vote-registered no recibió el evento (%d → %d)", auditBefore, got)
	}
}

// El reintento del gateway con la misma Idempotency-Key no duplica el voto.
func TestRetryIsIdempotent(t *testing.T) {
	key := randomKey()
	for i := 0; i < 3; i++ {
		postVote(t, asGateway(key), vote(1, 3, "Camila Duarte")).want(t, 201)
	}
	if docs := findByKey(t, key); len(docs) != 1 {
		t.Fatalf("se guardaron %d votos con la misma llave", len(docs))
	}
	postVote(t, asGateway(key), vote(1, 1, "Ana Torres")).want(t, 409)
}

func TestRejections(t *testing.T) {
	key := randomKey()
	postVote(t, map[string]string{"Idempotency-Key": key}, vote(1, 2, "Luis")).want(t, 401)
	postVote(t, map[string]string{"X-Service-Key": "otra", "Idempotency-Key": key}, vote(1, 2, "Luis")).want(t, 401)
	postVote(t, map[string]string{"X-Service-Key": serviceKey}, vote(1, 2, "Luis")).want(t, 422)
	postVote(t, asGateway(key), map[string]any{"election_id": 1}).want(t, 422)
	if docs := findByKey(t, key); len(docs) != 0 {
		t.Fatalf("un voto rechazado quedó guardado: %v", docs)
	}
}

// Outbox: un voto que quedó en published: false (por ejemplo, porque el broker
// estaba caído al guardarlo) se publica en la siguiente revisión.
func TestOutboxPublishesPendingVotes(t *testing.T) {
	events := listen(t)
	id := randomKey() + "0000aaaa" // 24 caracteres
	_, err := votes(t).InsertOne(context.Background(), bson.M{
		"_id": id, "election_id": 1, "candidate_id": 1, "candidate_name": "Ana Torres",
		"cast_at": time.Now().UTC().Truncate(time.Hour), "published": false, "idempotency_key": randomKey(),
	})
	if err != nil {
		t.Fatal(err)
	}
	waitEvent(t, events, id)
	waitPublished(t, id)
}
