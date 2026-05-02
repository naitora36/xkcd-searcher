package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"yadro.com/course/config"
	"yadro.com/course/handler"
)

func main() {
	cfg, err := config.GetConfigPort()
	if err != nil {
		slog.Error("config error", "error", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", handler.Ping)
	mux.HandleFunc("GET /hello", handler.Hello)

	server := http.Server{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
		Addr:         ":" + cfg.Port,
		Handler:      mux,
	}
	slog.Info("server starting", "addr", server.Addr)

	err = server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		slog.Error("server fatal error:", "error", err)
		os.Exit(1)
	}
}
