package main

import (
	"context"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"

	"dataset-tracker/internal/auth"
	"dataset-tracker/internal/db"
	"dataset-tracker/internal/handlers"
	"dataset-tracker/internal/middleware"
	"dataset-tracker/internal/models"
)

const logPath = "./data/bob.log"

func main() {
	devMode := os.Getenv("DEV_MODE") == "TRUE"

	if err := os.MkdirAll("./data", 0755); err != nil {
		log.Fatal("create data dir:", err)
	}

	// Dev mode: truncate the log on every restart. Production: append.
	logFlags := os.O_CREATE | os.O_WRONLY
	if devMode {
		logFlags |= os.O_TRUNC
	} else {
		logFlags |= os.O_APPEND
	}
	logFile, err := os.OpenFile(logPath, logFlags, 0644)
	if err != nil {
		log.Fatal("open log file:", err)
	}
	defer logFile.Close()

	logger := slog.New(slog.NewTextHandler(
		io.MultiWriter(os.Stdout, logFile),
		&slog.HandlerOptions{Level: slog.LevelInfo},
	))
	slog.SetDefault(logger)

	database, err := db.Init("./data/requests.db")
	if err != nil {
		log.Fatal("init db:", err)
	}
	defer database.Close()

	if devMode {
		slog.Warn("⚠  DEV_MODE enabled — CERN SSO is bypassed, do NOT use in production")
	}

	// OIDC client — optional so the app still starts without credentials configured,
	// but login will return a 503 until env vars are set.
	var oidcClient *auth.Client
	if os.Getenv("OIDC_CLIENT_ID") != "" {
		oidcClient, err = auth.NewClient(context.Background())
		if err != nil {
			log.Fatal("init OIDC:", err)
		}
		slog.Info("CERN SSO OIDC configured", "issuer", auth.CERNIssuer)
	} else {
		slog.Warn("OIDC_CLIENT_ID not set — CERN SSO login will be unavailable")
	}

	userRepo := models.NewUserStore(database)
	h := handlers.New(database, oidcClient, devMode)

	authMW := middleware.Auth(userRepo)

	mux := http.NewServeMux()

	// Static assets — public
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	// Auth — public
	mux.HandleFunc("GET /login", h.ShowLogin)
	mux.HandleFunc("GET /auth/login", h.Login)
	mux.HandleFunc("GET /auth/callback", h.Callback)
	mux.HandleFunc("POST /auth/dev-login", h.DevLogin) // only active when DEV_MODE=true
	mux.HandleFunc("GET /logout", h.Logout)

	// Protected routes — require authenticated session
	mux.HandleFunc("GET /", middleware.RequireAuth(h.Dashboard))
	mux.HandleFunc("GET /requests/new", middleware.RequireAuth(h.NewRequestForm))
	mux.HandleFunc("GET /requests", middleware.RequireAuth(h.ListRequests))
	mux.HandleFunc("POST /requests", middleware.RequireAuth(h.CreateRequest))
	mux.HandleFunc("GET /requests/{id}", middleware.RequireAuth(h.GetRequest))
	mux.HandleFunc("GET /requests/{id}/edit", middleware.RequireAuth(h.EditRequestForm))
	mux.HandleFunc("POST /requests/{id}", middleware.RequireAuth(h.UpdateRequest))
	mux.HandleFunc("GET /api/stats", middleware.RequireAuth(h.GetStats))

	// Manager-only routes
	mux.HandleFunc("GET /manager", middleware.RequireManager(h.ManagerView))
	mux.HandleFunc("POST /requests/batch", middleware.RequireManager(h.BatchAction))
	mux.HandleFunc("POST /requests/{id}/status", middleware.RequireManager(h.UpdateStatus))
	mux.HandleFunc("POST /requests/{id}/priority", middleware.RequireManager(h.UpdatePriority))
	mux.HandleFunc("POST /requests/{id}/assign", middleware.RequireManager(h.AssignRequest))
	mux.HandleFunc("POST /requests/{id}/comments", middleware.RequireAuth(h.AddComment))
	mux.HandleFunc("DELETE /requests/{id}", middleware.RequireManager(h.DeleteRequest))

	addr := ":5050"
	slog.Info("server started", "addr", "http://localhost"+addr)
	if err := http.ListenAndServe(addr, authMW(mux)); err != nil {
		log.Fatal(err)
	}
}
