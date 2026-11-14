package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/log"

	"github.com/untittledgoosegame/core-backend/internal/api/handlers"
	"github.com/untittledgoosegame/core-backend/internal/api/routes"
	"github.com/untittledgoosegame/core-backend/internal/auth"
	"github.com/untittledgoosegame/core-backend/internal/connection"
	"github.com/untittledgoosegame/core-backend/internal/server"
	"github.com/untittledgoosegame/core-backend/internal/user"
	"github.com/untittledgoosegame/core-backend/pkg/cache"
	"github.com/untittledgoosegame/core-backend/pkg/config"
	"github.com/untittledgoosegame/core-backend/pkg/database"
	"github.com/untittledgoosegame/core-backend/pkg/logger"
	"github.com/untittledgoosegame/core-backend/pkg/mailer"
)

func main() {
	if err := config.LoadConfig(); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	cfg := config.AppConfig
	logger.Setup(cfg.Log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	client, err := database.Connect(ctx, cfg.Mongo.URI)
	if err != nil {
		log.Fatalf("Failed to connect to mongo: %v", err)
	}
	defer client.Disconnect(context.Background())
	db := client.Database(cfg.Mongo.Database)

	rdb, err := cache.Connect(ctx, cfg.Redis)
	if err != nil {
		log.Fatalf("Failed to connect to redis: %v", err)
	}
	defer rdb.Close()

	users := user.NewStore(db)
	if err := users.EnsureIndexes(ctx); err != nil {
		log.Fatalf("Failed to create user indexes: %v", err)
	}

	connections := connection.NewStore(db)
	if err := connections.EnsureIndexes(ctx); err != nil {
		log.Fatalf("Failed to create connection indexes: %v", err)
	}

	mail := mailer.New(cfg.SMTP, cfg.App.Name, cfg.IsDev())
	authService, err := auth.NewService(ctx, db, rdb, users, mail, cfg)
	if err != nil {
		log.Fatalf("Failed to set up auth: %v", err)
	}

	h := handlers.New(handlers.Deps{
		DB:          db,
		Redis:       rdb,
		Users:       users,
		Connections: connections,
		Auth:        authService,
		Config:      cfg,
	})
	router := routes.New(h, authService.Tokens(), cfg)
	srv := server.New(net.JoinHostPort(cfg.Server.Host, cfg.Server.Port), router)
	go func() {
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Info("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Errorf("Server forced to shutdown: %v", err)
	}
	log.Info("Server exited")
}
