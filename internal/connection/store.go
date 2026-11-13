package connection

import (
	"context"
	"errors"
	"time"

	"github.com/untittledgoosegame/core-backend/internal/provider"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Store struct {
	connections *mongo.Collection
}

func NewStore(db *mongo.Database) *Store {
	return &Store{connections: db.Collection("connections")}
}

func (s *Store) EnsureIndexes(ctx context.Context) error {
	_, err := s.connections.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "provider", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return err
}

// Apply stores an event unless a newer one for the same user and provider was already applied.
// It reports false for stale events. Ordering comes from occurred_at, not arrival time.
func (s *Store) Apply(ctx context.Context, userID bson.ObjectID, e Event) (bool, error) {
	now := time.Now().UTC()
	occurred := e.OccurredAt.UTC()

	set := bson.D{
		{Key: "connected", Value: e.Type != EventDisconnected},
		{Key: "last_event_id", Value: e.ID},
		{Key: "last_event_at", Value: occurred},
		{Key: "updated_at", Value: now},
	}
	var unset bson.D
	switch e.Type {
	case EventDisconnected:
		set = append(set, bson.E{Key: "disconnected_at", Value: occurred})
		unset = bson.D{{Key: "account", Value: ""}, {Key: "connected_at", Value: ""}}
	case EventConnected:
		set = append(set, bson.E{Key: "account", Value: e.Account}, bson.E{Key: "connected_at", Value: occurred})
		unset = bson.D{{Key: "disconnected_at", Value: ""}}
	case EventUpdated:
		set = append(set, bson.E{Key: "account", Value: e.Account})
		unset = bson.D{{Key: "disconnected_at", Value: ""}}
	}

	update := bson.D{{Key: "$set", Value: set}}
	if len(unset) > 0 {
		update = append(update, bson.E{Key: "$unset", Value: unset})
	}
	if e.Type == EventUpdated {
		update = append(update, bson.E{Key: "$setOnInsert", Value: bson.D{{Key: "connected_at", Value: occurred}}})
	}

	filter := bson.D{
		{Key: "user_id", Value: userID},
		{Key: "provider", Value: e.Provider},
		{Key: "$or", Value: bson.A{
			bson.D{{Key: "last_event_at", Value: bson.D{{Key: "$lt", Value: occurred}}}},
			bson.D{{Key: "last_event_at", Value: bson.D{{Key: "$exists", Value: false}}}},
		}},
	}
	_, err := s.connections.UpdateOne(ctx, filter, update, options.UpdateOne().SetUpsert(true))
	if mongo.IsDuplicateKeyError(err) {
		// The record exists but holds a newer event, so the upsert tried to insert a second one.
		return false, nil
	}
	return err == nil, err
}

func (s *Store) Get(ctx context.Context, userID bson.ObjectID, p provider.Provider) (*Connection, error) {
	var c Connection
	err := s.connections.FindOne(ctx, bson.D{{Key: "user_id", Value: userID}, {Key: "provider", Value: p}}).Decode(&c)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ListForUser returns every provider in display order, connected or not.
func (s *Store) ListForUser(ctx context.Context, userID bson.ObjectID) ([]View, error) {
	cur, err := s.connections.Find(ctx, bson.D{{Key: "user_id", Value: userID}})
	if err != nil {
		return nil, err
	}
	var stored []Connection
	if err := cur.All(ctx, &stored); err != nil {
		return nil, err
	}
	byProvider := make(map[provider.Provider]Connection, len(stored))
	for _, c := range stored {
		byProvider[c.Provider] = c
	}

	views := make([]View, 0, len(provider.All))
	for _, p := range provider.All {
		c, ok := byProvider[p]
		if !ok {
			views = append(views, View{Provider: p})
			continue
		}
		views = append(views, c.View())
	}
	return views, nil
}
