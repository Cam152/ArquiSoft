// Package repository es el acceso a la Votes DB (MongoDB).
package repository

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"vote-service/internal/model"
)

// MongoStore guarda los votos y la auditoría en MongoDB.
type MongoStore struct {
	client *mongo.Client
	votes  *mongo.Collection
	audit  *mongo.Collection
}

const (
	connectRetries = 30
	connectDelay   = time.Second
)

// NewMongoStore espera a que MongoDB responda y crea el índice de votos.
func NewMongoStore(ctx context.Context, uri, database string) (*MongoStore, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
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
		if attempt == connectRetries {
			return nil, fmt.Errorf("MongoDB no respondió tras %d intentos: %w", attempt, err)
		}
		log.Printf("esperando a MongoDB (intento %d/%d)", attempt, connectRetries)
		time.Sleep(connectDelay)
	}

	db := client.Database(database)
	store := &MongoStore{client: client, votes: db.Collection("votes"), audit: db.Collection("audit")}
	_, err = store.votes.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "election_id", Value: 1}, {Key: "candidate_id", Value: 1}},
	})
	if err != nil {
		return nil, err
	}
	return store, nil
}

func (s *MongoStore) InsertVote(ctx context.Context, vote model.Vote) error {
	_, err := s.votes.InsertOne(ctx, vote)
	return err
}

func (s *MongoStore) InsertAudit(ctx context.Context, event model.AuditEvent) error {
	_, err := s.audit.InsertOne(ctx, event)
	return err
}

// CountByCandidate devuelve candidate_id -> número de votos.
func (s *MongoStore) CountByCandidate(ctx context.Context, electionID int) (map[int]int, error) {
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

func (s *MongoStore) Close(ctx context.Context) error {
	return s.client.Disconnect(ctx)
}
