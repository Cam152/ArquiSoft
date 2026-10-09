package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Vote es un voto anónimo: no lleva ningún dato del votante.
type Vote struct {
	ElectionID    int       `bson:"election_id"`
	CandidateID   int       `bson:"candidate_id"`
	CandidateName string    `bson:"candidate_name"`
	CastAt        time.Time `bson:"cast_at"`
}

// AuditEvent tampoco lleva datos del votante.
type AuditEvent struct {
	Type       string    `bson:"type"`
	ElectionID int       `bson:"election_id"`
	At         time.Time `bson:"at"`
}

// Store es el acceso a la Votes DB.
type Store interface {
	InsertVote(ctx context.Context, vote Vote) error
	InsertAudit(ctx context.Context, event AuditEvent) error
	// CountByCandidate devuelve candidate_id -> número de votos.
	CountByCandidate(ctx context.Context, electionID int) (map[int]int, error)
}

type mongoStore struct {
	client *mongo.Client
	votes  *mongo.Collection
	audit  *mongo.Collection
}

const (
	mongoConnectRetries = 30
	mongoConnectDelay   = time.Second
)

// newMongoStore espera a que MongoDB responda y crea el índice de votos.
func newMongoStore(ctx context.Context, cfg Config) (*mongoStore, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		return nil, err
	}
	for attempt := 1; ; attempt++ {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err = client.Ping(pingCtx, nil)
		cancel()
		if err == nil {
			break
		}
		if attempt == mongoConnectRetries {
			return nil, fmt.Errorf("MongoDB no respondió tras %d intentos: %w", attempt, err)
		}
		log.Printf("esperando a MongoDB (intento %d/%d)", attempt, mongoConnectRetries)
		time.Sleep(mongoConnectDelay)
	}

	db := client.Database(cfg.MongoDB)
	store := &mongoStore{client: client, votes: db.Collection("votes"), audit: db.Collection("audit")}
	_, err = store.votes.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "election_id", Value: 1}, {Key: "candidate_id", Value: 1}},
	})
	if err != nil {
		return nil, err
	}
	return store, nil
}

func (s *mongoStore) InsertVote(ctx context.Context, vote Vote) error {
	_, err := s.votes.InsertOne(ctx, vote)
	return err
}

func (s *mongoStore) InsertAudit(ctx context.Context, event AuditEvent) error {
	_, err := s.audit.InsertOne(ctx, event)
	return err
}

func (s *mongoStore) CountByCandidate(ctx context.Context, electionID int) (map[int]int, error) {
	cursor, err := s.votes.Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: bson.D{{Key: "election_id", Value: electionID}}}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$candidate_id"},
			{Key: "votes", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
	})
	if err != nil {
		return nil, err
	}
	var rows []struct {
		CandidateID int `bson:"_id"`
		Votes       int `bson:"votes"`
	}
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, err
	}
	counts := make(map[int]int, len(rows))
	for _, row := range rows {
		counts[row.CandidateID] = row.Votes
	}
	return counts, nil
}

func (s *mongoStore) Close(ctx context.Context) error {
	return s.client.Disconnect(ctx)
}
