package router

import (
	"compress/gzip"
	"database/sql"

	"github.com/ustasjs/gopher-markt/internal/config/settings"
	"github.com/ustasjs/gopher-markt/internal/handler"
	"github.com/ustasjs/gopher-markt/internal/logger"
	customMiddleware "github.com/ustasjs/gopher-markt/internal/middleware"
	"github.com/ustasjs/gopher-markt/internal/service"
	"github.com/ustasjs/gopher-markt/internal/storage"
	"github.com/ustasjs/gopher-markt/migrations"

	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"
)

func StartServer() {
	settingsMap := settings.InitSettings()
	loggerErr := logger.Initialize(settingsMap.LogLevel)
	if loggerErr != nil {
		panic(loggerErr)
	}

	if settingsMap.DatabaseURI == "" {
		panic("DATABASE_URI is required")
	}

	db, dbErr := sql.Open("pgx", string(settingsMap.DatabaseURI))
	if dbErr != nil {
		panic(dbErr)
	}
	defer db.Close()

	logger.Log.Info("Connect to database")

	if pingErr := db.Ping(); pingErr != nil {
		panic(pingErr)
	}

	migrationsErr := migrations.RunMigrations(db)
	if migrationsErr != nil {
		panic(migrationsErr)
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

	err := srv.ListenAndServe()
	if err != nil {
		panic(err)
	}
}

func initRoutes(r *chi.Mux, store storage.Repository, jwtService *service.JWTService) {
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
	})
}

func initMiddleware(r *chi.Mux, parser customMiddleware.TokenParser) {
	r.Use(logger.LoggerMiddleware)
	r.Use(customMiddleware.GzipDecompress)
	r.Use(middleware.Compress(gzip.DefaultCompression))
	r.Use(customMiddleware.Auth(parser))
}
