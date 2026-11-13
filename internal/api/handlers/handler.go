package handlers

import (
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/untittledgoosegame/core-backend/internal/auth"
	"github.com/untittledgoosegame/core-backend/internal/connection"
	"github.com/untittledgoosegame/core-backend/internal/user"
	"github.com/untittledgoosegame/core-backend/pkg/config"
)

type Deps struct {
	DB          *mongo.Database
	Redis       *redis.Client
	Users       *user.Store
	Connections *connection.Store
	Auth        *auth.Service
	Config      config.Config
}

type Handler struct {
	db          *mongo.Database
	redis       *redis.Client
	users       *user.Store
	connections *connection.Store
	auth        *auth.Service
	mobile      config.MobileConfig
	webhooks    config.WebhooksConfig
}

func New(d Deps) *Handler {
	return &Handler{
		db:          d.DB,
		redis:       d.Redis,
		users:       d.Users,
		connections: d.Connections,
		auth:        d.Auth,
		mobile:      d.Config.Mobile,
		webhooks:    d.Config.Webhooks,
	}
}
