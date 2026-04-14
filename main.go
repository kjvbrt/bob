package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"

	"dataset-tracker/internal/db"
	"dataset-tracker/internal/handlers"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	database, err := db.Init("./data/requests.db")
	if err != nil {
		log.Fatal("init db:", err)
	}
	defer database.Close()

	h := handlers.New(database)

	mux := http.NewServeMux()

	// Static assets
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	// Dashboard
	mux.HandleFunc("GET /", h.Dashboard)

	// Requests – order matters: literals before wildcards
	mux.HandleFunc("GET /requests/new", h.NewRequestForm)
	mux.HandleFunc("GET /requests", h.ListRequests)
	mux.HandleFunc("POST /requests", h.CreateRequest)
	mux.HandleFunc("GET /requests/{id}", h.GetRequest)
	mux.HandleFunc("GET /requests/{id}/edit", h.EditRequestForm)
	mux.HandleFunc("POST /requests/{id}", h.UpdateRequest)
	mux.HandleFunc("POST /requests/{id}/status", h.UpdateStatus)
	mux.HandleFunc("DELETE /requests/{id}", h.DeleteRequest)

	// Stats for HTMX polling
	mux.HandleFunc("GET /api/stats", h.GetStats)

	addr := ":5050"
	slog.Info("server started", "addr", "http://localhost"+addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
