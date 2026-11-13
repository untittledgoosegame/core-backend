package connection

import (
	"context"
	"errors"
	"time"

	"github.com/charmbracelet/log"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/untittledgoosegame/core-backend/internal/provider"
	"github.com/untittledgoosegame/core-backend/internal/user"
	"github.com/untittledgoosegame/core-backend/pkg/apperr"
	"github.com/untittledgoosegame/core-backend/pkg/config"
)

const eventDedupTTL = 7 * 24 * time.Hour

type EventStatus string

const (
	StatusApplied     EventStatus = "applied"
	StatusDuplicate   EventStatus = "duplicate"
	StatusStale       EventStatus = "stale"
	StatusUnknownUser EventStatus = "ignored_unknown_user"
)

var (
	errUnknownProvider = apperr.NotFound("unknown provider")
	errNotConnected    = apperr.NotFound("this provider is not connected")
	errAlreadyLinked   = apperr.Conflict("this provider is already connected; disconnect it first")
)

type ConnectLink struct {
	Provider   provider.Provider `json:"provider"`
	ConnectURL string            `json:"connect_url"`
	ExpiresAt  time.Time         `json:"expires_at"`
}

type Service struct {
	store  *Store
	users  *user.Store
	redis  *redis.Client
	oauth  *oauthService
	secret string
}

func NewService(store *Store, users *user.Store, rdb *redis.Client, cfg config.OAuthServiceConfig) *Service {
	return &Service{store: store, users: users, redis: rdb, oauth: newOAuthService(cfg), secret: cfg.Secret}
}

func (s *Service) WebhookSecret() string { return s.secret }

func parseProvider(raw string) (provider.Provider, error) {
	p := provider.Provider(raw)
	if !p.Valid() {
		return "", errUnknownProvider
	}
	return p, nil
}

func (s *Service) List(ctx context.Context, userID bson.ObjectID) ([]View, error) {
	return s.store.ListForUser(ctx, userID)
}

func (s *Service) Get(ctx context.Context, userID bson.ObjectID, rawProvider string) (*View, error) {
	p, err := parseProvider(rawProvider)
	if err != nil {
		return nil, err
	}
	c, err := s.store.Get(ctx, userID, p)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return &View{Provider: p}, nil
	}
	v := c.View()
	return &v, nil
}

func (s *Service) Connect(ctx context.Context, userID bson.ObjectID, rawProvider string) (*ConnectLink, error) {
	p, err := parseProvider(rawProvider)
	if err != nil {
		return nil, err
	}
	c, err := s.store.Get(ctx, userID, p)
	if err != nil {
		return nil, err
	}
	if c != nil && c.Connected {
		return nil, errAlreadyLinked
	}
	link, expires, err := s.oauth.connectURL(userID, p, time.Now())
	if err != nil {
		return nil, err
	}
	return &ConnectLink{Provider: p, ConnectURL: link, ExpiresAt: expires}, nil
}

// Disconnect revokes at the OAuth service first; local data is only cleared once it confirms,
// otherwise its next "updated" webhook would silently reconnect the account.
func (s *Service) Disconnect(ctx context.Context, userID bson.ObjectID, rawProvider string) (*View, error) {
	p, err := parseProvider(rawProvider)
	if err != nil {
		return nil, err
	}
	c, err := s.store.Get(ctx, userID, p)
	if err != nil {
		return nil, err
	}
	if c == nil || !c.Connected {
		return nil, errNotConnected
	}
	if err := s.oauth.revoke(ctx, userID, p); err != nil {
		return nil, err
	}

	event := Event{ID: "user:" + bson.NewObjectID().Hex(), Type: EventDisconnected, OccurredAt: time.Now().UTC(), Provider: p}
	if _, err := s.apply(ctx, userID, event); err != nil {
		return nil, err
	}
	return &View{Provider: p}, nil
}

// HandleEvent applies a verified webhook event. Every outcome except an error is final for the sender.
func (s *Service) HandleEvent(ctx context.Context, e Event) (EventStatus, error) {
	userID, err := bson.ObjectIDFromHex(e.UserID)
	if err != nil {
		return "", apperr.BadRequest("user_id is not a valid id")
	}
	if _, err := s.users.ByID(ctx, userID); errors.Is(err, user.ErrNotFound) {
		log.Warn("webhook for unknown user", "event_id", e.ID, "user_id", e.UserID)
		return StatusUnknownUser, nil
	} else if err != nil {
		return "", err
	}

	dedupKey := "webhook:oauth:" + e.ID
	fresh, err := s.redis.SetNX(ctx, dedupKey, 1, eventDedupTTL).Result()
	if err != nil {
		return "", err
	}
	if !fresh {
		return StatusDuplicate, nil
	}
	status, err := s.apply(ctx, userID, e)
	if err != nil {
		s.redis.Del(ctx, dedupKey)
		return "", err
	}
	return status, nil
}

// apply stores the event, then copies the resulting connected flag onto the user.
// The flag is synced even for stale events so a failed earlier sync heals on retry.
func (s *Service) apply(ctx context.Context, userID bson.ObjectID, e Event) (EventStatus, error) {
	applied, err := s.store.Apply(ctx, userID, e)
	if err != nil {
		return "", err
	}
	current, err := s.store.Get(ctx, userID, e.Provider)
	if err != nil {
		return "", err
	}
	if err := s.users.SetConnectionFlag(ctx, userID, e.Provider, current != nil && current.Connected); err != nil {
		return "", err
	}
	if !applied {
		return StatusStale, nil
	}
	return StatusApplied, nil
}
