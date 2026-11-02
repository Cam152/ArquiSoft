package publisher

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"vote-service/internal/model"
)

// El mensaje cumple el contrato de VoteRegistered v1 (README, sección 5.3) y no
// lleva nada más: ni datos del votante, ni candidate_name, ni la hora exacta.
func TestEncodeFollowsTheContract(t *testing.T) {
	vote := model.Vote{
		ID: "6720f3a1c2b9e8a4d1f00a17", ElectionID: 1, CandidateID: 2, CandidateName: "Luis Herrera",
		CastAt:         time.Date(2026, 11, 3, 9, 0, 0, 0, time.FixedZone("COT", -5*3600)),
		IdempotencyKey: "llave-secreta",
	}
	msg, err := Encode(model.EventFor(vote))
	if err != nil {
		t.Fatal(err)
	}

	if msg.ContentType != "application/json" || msg.DeliveryMode != amqp.Persistent ||
		msg.MessageId != vote.ID || msg.Type != "VoteRegistered" || !msg.Timestamp.IsZero() {
		t.Fatalf("propiedades AMQP inesperadas: %+v", msg)
	}

	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		t.Fatal(err)
	}
	var fields []string
	for k := range body {
		fields = append(fields, k)
	}
	slices.Sort(fields)
	want := []string{"candidate_id", "election_id", "event_id", "occurred_at", "type", "version"}
	if !slices.Equal(fields, want) {
		t.Fatalf("campos %v, se esperaba %v", fields, want)
	}
	if body["event_id"] != vote.ID || body["type"] != "VoteRegistered" || body["version"] != 1.0 ||
		body["election_id"] != 1.0 || body["candidate_id"] != 2.0 {
		t.Fatalf("valores inesperados: %v", body)
	}
	if body["occurred_at"] != "2026-11-03T14:00:00Z" {
		t.Fatalf("occurred_at = %v, se esperaba 2026-11-03T14:00:00Z (UTC)", body["occurred_at"])
	}
}
