package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/untittledgoosegame/core-backend/pkg/apperr"
)

const (
	challengeTTL = 5 * time.Minute
	maxAttempts  = 5
)

type challengeDoc struct {
	ID        string        `bson:"_id"`
	Kind      string        `bson:"kind"`
	Session   string        `bson:"session,omitempty"`
	UserID    bson.ObjectID `bson:"user_id,omitempty"`
	Attempts  int           `bson:"attempts"`
	ExpiresAt time.Time     `bson:"expires_at"`
}

type challengeStore struct {
	challenges *mongo.Collection
}

func newChallengeStore(ctx context.Context, db *mongo.Database) (*challengeStore, error) {
	s := &challengeStore{challenges: db.Collection("auth_challenges")}
	ttl := mongo.IndexModel{Keys: bson.D{{Key: "expires_at", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)}
	if _, err := s.challenges.Indexes().CreateOne(ctx, ttl); err != nil {
		return nil, err
	}
	return s, nil
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *challengeStore) createChallenge(ctx context.Context, doc challengeDoc) (string, error) {
	id, err := randomToken(32)
	if err != nil {
		return "", err
	}
	doc.ID = id
	doc.ExpiresAt = time.Now().UTC().Add(challengeTTL)
	_, err = s.challenges.InsertOne(ctx, doc)
	return id, err
}

func (s *challengeStore) takeChallenge(ctx context.Context, id, kind string) (*challengeDoc, error) {
	var doc challengeDoc
	err := s.challenges.FindOneAndDelete(ctx, bson.D{
		{Key: "_id", Value: id},
		{Key: "kind", Value: kind},
		{Key: "expires_at", Value: bson.D{{Key: "$gt", Value: time.Now().UTC()}}},
	}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, apperr.Unauthorized("challenge expired, start again")
	}
	return &doc, err
}

func (s *challengeStore) attemptChallenge(ctx context.Context, id, kind string) (*challengeDoc, error) {
	var doc challengeDoc
	err := s.challenges.FindOneAndUpdate(ctx,
		bson.D{
			{Key: "_id", Value: id},
			{Key: "kind", Value: kind},
			{Key: "expires_at", Value: bson.D{{Key: "$gt", Value: time.Now().UTC()}}},
			{Key: "attempts", Value: bson.D{{Key: "$lt", Value: maxAttempts}}},
		},
		bson.D{{Key: "$inc", Value: bson.D{{Key: "attempts", Value: 1}}}},
	).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, apperr.Unauthorized("challenge expired or too many attempts, log in again")
	}
	return &doc, err
}

func (s *challengeStore) deleteChallenge(ctx context.Context, id string) error {
	_, err := s.challenges.DeleteOne(ctx, bson.D{{Key: "_id", Value: id}})
	return err
}
