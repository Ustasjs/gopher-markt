package router

import (
	"compress/gzip"
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"

	"github.com/ustasjs/gopher-markt/internal/accrual"
	"github.com/ustasjs/gopher-markt/internal/config/settings"
	"github.com/ustasjs/gopher-markt/internal/handler"
	"github.com/ustasjs/gopher-markt/internal/logger"
	customMiddleware "github.com/ustasjs/gopher-markt/internal/middleware"
	"github.com/ustasjs/gopher-markt/internal/service"
	"github.com/ustasjs/gopher-markt/internal/storage"
	"github.com/ustasjs/gopher-markt/migrations"
)

func StartServer() error {
	settingsMap, err := settings.InitSettings()
	if err != nil {
		return err
	}

	if err = logger.Initialize(settingsMap.LogLevel); err != nil {
		return err
	}

	if settingsMap.DatabaseURI == "" {
		return errors.New("DATABASE_URI is required")
	}

	db, err := sql.Open("pgx", string(settingsMap.DatabaseURI))
	if err != nil {
		return err
	}
	defer db.Close()

	logger.Log.Info("Connect to database")

	if err = db.Ping(); err != nil {
		return err
	}

	if err = migrations.RunMigrations(db); err != nil {
		return err
	}

	logger.Log.Info("Starting server on:", zap.String("address", string(settingsMap.ServerAddress)))

	var store = storage.NewPostgresRepository(db)

	r := chi.NewRouter()
	jwtService := service.NewJWTService([]byte(settingsMap.JWTSecret))
	initMiddleware(r, jwtService)
	initRoutes(r, store, jwtService)

	srv := &http.Server{
		Addr:              string(settingsMap.ServerAddress),
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	accrualClient := accrual.NewClient(string(settingsMap.AccrualSystemAddress))
	pool := accrual.NewWorkerPool(store, accrualClient)
	poolDone := make(chan struct{})
	go func() {
		defer close(poolDone)
		pool.Run(ctx)
	}()

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Fatal("server error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	stop()
	logger.Log.Info("shutting down server")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Log.Error("server shutdown error", zap.Error(err))
	}

	<-poolDone
	logger.Log.Info("accrual worker pool stopped")
	return nil
}

func initRoutes(r *chi.Mux, store *storage.PostgresRepository, jwtService *service.JWTService) {
	userService := service.NewUserService(store, jwtService)
	userHandler := handler.NewUserHandler(userService)

	orderService := service.NewOrderService(store)
	orderHandler := handler.NewOrderHandler(orderService)

	balanceService := service.NewBalanceService(store)
	balanceHandler := handler.NewBalanceHandler(balanceService)

	r.Post("/api/user/register", userHandler.Register)
	r.Post("/api/user/login", userHandler.Login)

	r.Group(func(r chi.Router) {
		r.Use(customMiddleware.RequireAuth())
		r.Post("/api/user/orders", orderHandler.UploadOrder)
		r.Get("/api/user/orders", orderHandler.GetOrders)
		r.Get("/api/user/balance", balanceHandler.GetBalance)
		r.Post("/api/user/balance/withdraw", balanceHandler.Withdraw)
		r.Get("/api/user/withdrawals", balanceHandler.GetWithdrawals)
	})
}

func initMiddleware(r *chi.Mux, parser customMiddleware.TokenParser) {
	r.Use(customMiddleware.LoggerMiddleware)
	r.Use(customMiddleware.GzipDecompress)
	r.Use(middleware.Compress(gzip.DefaultCompression))
	r.Use(customMiddleware.Auth(parser))
}
