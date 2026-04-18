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

	var db *sql.DB
	if settingsMap.DatabaseURI != "" {
		var dbErr error
		db, dbErr = sql.Open("pgx", string(settingsMap.DatabaseURI))

		logger.Log.Info("Connect to database")

		if dbErr != nil {
			panic(dbErr)
		}
		defer db.Close()

		migrationsErr := migrations.RunMigrations(db)
		if migrationsErr != nil {
			panic(migrationsErr)
		}
	}

	logger.Log.Info("Starting server on:", zap.String("address", string(settingsMap.ServerAddress)))

	var store = storage.NewPostgresRepository(db)

	r := chi.NewRouter()
	initMiddleware(r, store)
	initRoutes(r, settingsMap, store)

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

func initRoutes(r *chi.Mux, s *settings.Settings, store storage.Repository) {
	jwtService := service.NewJWTService([]byte(s.JWTSecret))
	userService := service.NewUserService(store, jwtService)

	userHandler := handler.NewUserHandler(userService)

	// Register routes
	r.Post("/api/user/register", userHandler.Register)
	r.Post("/api/user/login", userHandler.Login)
}

func initMiddleware(r *chi.Mux, store customMiddleware.UserRepository) {
	r.Use(logger.LoggerMiddleware)
	r.Use(customMiddleware.GzipDecompress)
	r.Use(middleware.Compress(gzip.DefaultCompression))
	r.Use(customMiddleware.Auth(store))
}
