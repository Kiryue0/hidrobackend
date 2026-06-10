package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	apphttp "github.com/kiryue0/hidrobackend/internal/infra/http"

	"github.com/kiryue0/hidrobackend/internal/app/usecases"
	"github.com/kiryue0/hidrobackend/internal/infra/config"
	"github.com/kiryue0/hidrobackend/internal/infra/migrate"
	"github.com/kiryue0/hidrobackend/internal/infra/mqtt"
	"github.com/kiryue0/hidrobackend/internal/infra/postgres"
	"github.com/kiryue0/hidrobackend/internal/infra/postgres/db"
	"github.com/kiryue0/hidrobackend/internal/infra/security"
	"github.com/kiryue0/hidrobackend/internal/infra/ws"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config yuklenemedi", "err", err)
		os.Exit(1)
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("db pool olusturulamadi", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		logger.Error("db ping basarisiz", "err", err)
		os.Exit(1)
	}

	// Otomatik migration (idempotent, baslangicta calisir)
	if err := migrate.Run(ctx, pool, cfg.MigrationsDir, logger); err != nil {
		logger.Error("migration hatasi", "err", err)
		os.Exit(1)
	}

	// --- Bagimlilik montaji (composition root) ---
	queries := db.New(pool)
	userRepo := postgres.NewUserRepo(queries)
	cabinRepo := postgres.NewCabinRepo(pool, queries)
	readingRepo := postgres.NewReadingRepo(queries)
	alertRepo := postgres.NewAlertRepo(queries)
	hasher := security.NewBcryptHasher(0)
	tokens := security.NewJWTService(cfg.JWTSecret, cfg.JWTTTL)

	// WebSocket hub (LiveBroadcastPort)
	hub := ws.NewHub(logger)
	go hub.Run(ctx)

	authService := usecases.NewAuthService(userRepo, cabinRepo, hasher, tokens)
	authHandler := apphttp.NewAuthHandler(authService)
	cabinService := usecases.NewCabinService(cabinRepo)
	cabinHandler := apphttp.NewCabinHandler(cabinService)
	wsHandler := apphttp.NewWSHandler(hub, tokens, cabinService, cfg.AllowedOriginList())
	telemetryService := usecases.NewTelemetryService(cabinRepo, readingRepo, alertRepo, userRepo, hub, logger)
	historyService := usecases.NewHistoryService(cabinRepo, readingRepo)
	historyHandler := apphttp.NewHistoryHandler(historyService)

	// Retention: eski ham telemetriyi temizler.
	go usecases.NewRetentionService(readingRepo, cfg.ReadingRetentionDays, logger).Run(ctx)

	mqttCfg := mqtt.Config{
		Broker:   cfg.MQTTBroker,
		ClientID: cfg.MQTTClientID,
		Username: cfg.MQTTUsername,
		Password: cfg.MQTTPassword,
	}

	// MQTT subscriber (cihaz -> backend)
	// Hata fatal degil: AutoReconnect+ConnectRetry arka planda yeniden dener.
	subscriber := mqtt.NewSubscriber(mqttCfg, telemetryService, logger)
	if err := subscriber.Start(ctx); err != nil {
		logger.Warn("mqtt subscriber baslatilamadi, arka planda yeniden deneniyor", "err", err)
	}
	defer subscriber.Stop()

	// MQTT publisher (backend -> cihaz): down/command + down/config
	publisher := mqtt.NewPublisher(mqttCfg, logger)
	if err := publisher.Start(ctx); err != nil {
		logger.Warn("mqtt publisher baslatilamadi, arka planda yeniden deneniyor", "err", err)
	}
	defer publisher.Stop()
	controlService := usecases.NewControlService(cabinRepo, publisher, publisher.ConfigPort())
	controlHandler := apphttp.NewControlHandler(controlService)

	gin.SetMode(gin.ReleaseMode)
	router := apphttp.NewRouter(apphttp.Deps{
		Auth:    authHandler,
		Cabin:   cabinHandler,
		Control: controlHandler,
		History: historyHandler,
		WS:      wsHandler,
		Tokens:  tokens,
		DB:      pool,
		WebDir:  cfg.WebDir,
	})

	srv := &http.Server{
		Addr:    ":" + cfg.HTTPPort,
		Handler: router,
	}

	go func() {
		logger.Info("http sunucu basliyor", "port", cfg.HTTPPort)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http sunucu hatasi", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("kapatiliyor...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown hatasi", "err", err)
	}
}
