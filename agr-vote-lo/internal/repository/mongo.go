// Package repository es el único acceso a agr-votes-db (MongoDB).
package repository

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"vote-service/internal/model"
)

// MongoStore guarda los votos en la colección votes.
type MongoStore struct {
	client *mongo.Client
	votes  *mongo.Collection
}

const (
	connectRetries = 30
	connectDelay   = time.Second
)

// NewMongoStore espera a que MongoDB responda y crea los índices.
func NewMongoStore(ctx context.Context, uri, database string) (*MongoStore, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri).SetAppName("agr-vote-lo"))
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
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(connectDelay):
		}
	}

	store := &MongoStore{client: client, votes: client.Database(database).Collection("votes")}
	if err := store.ensureIndexes(ctx); err != nil {
		return nil, fmt.Errorf("no se pudieron crear los índices: %w", err)
	}
	return store, nil
}

func (s *MongoStore) ensureIndexes(ctx context.Context) error {
	_, err := s.votes.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			// Un voto por Idempotency-Key: el reintento del gateway no lo duplica.
			Keys:    bson.D{{Key: "idempotency_key", Value: 1}},
			Options: options.Index().SetName("idempotency_key_unique").SetUnique(true),
		},
		{
			// Índice parcial: solo contiene los votos pendientes del outbox, así la
			// búsqueda periódica no recorre los ya publicados.
			Keys: bson.D{{Key: "published", Value: 1}},
			Options: options.Index().SetName("published_pending").
				SetPartialFilterExpression(bson.D{{Key: "published", Value: false}}),
		},
	})
	return err
}

// Insert guarda el voto. Devuelve model.ErrDuplicateKey si la llave ya existe.
func (s *MongoStore) Insert(ctx context.Context, vote model.Vote) error {
	_, err := s.votes.InsertOne(ctx, vote)
	if mongo.IsDuplicateKeyError(err) {
		return model.ErrDuplicateKey
	}
	return err
}

// FindByIdempotencyKey devuelve nil, nil si no existe.
func (s *MongoStore) FindByIdempotencyKey(ctx context.Context, key string) (*model.Vote, error) {
	var vote model.Vote
	err := s.votes.FindOne(ctx, bson.D{{Key: "idempotency_key", Value: key}}).Decode(&vote)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &vote, nil
}

// FindUnpublished devuelve hasta limit votos que siguen en published: false.
func (s *MongoStore) FindUnpublished(ctx context.Context, limit int) ([]model.Vote, error) {
	cursor, err := s.votes.Find(ctx,
		bson.D{{Key: "published", Value: false}},
		options.Find().SetLimit(int64(limit)).SetHint("published_pending"),
	)
	if err != nil {
		return nil, err
	}
	var votes []model.Vote
	if err := cursor.All(ctx, &votes); err != nil {
		return nil, err
	}
	return votes, nil
}

// MarkPublished marca el voto cuando el broker confirmó su evento.
func (s *MongoStore) MarkPublished(ctx context.Context, id string) error {
	_, err := s.votes.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: id}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "published", Value: true}}}},
	)
	return err
}

// Ping lo usa el health check.
func (s *MongoStore) Ping(ctx context.Context) error {
	return s.client.Ping(ctx, nil)
}

func (s *MongoStore) Close(ctx context.Context) error {
	return s.client.Disconnect(ctx)
}
