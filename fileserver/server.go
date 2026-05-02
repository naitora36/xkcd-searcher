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

	err = os.MkdirAll("file", 0o750)
	if err != nil {
		slog.Error("mkdir error", "error", err)
		os.Exit(1)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /files", handler.CreateFile)
	mux.HandleFunc("PUT /files/{filename}", handler.UpdateFile)
	mux.HandleFunc("GET /files", handler.ListFiles)
	mux.HandleFunc("GET /files/{filename}", handler.PrintFile)
	mux.HandleFunc("DELETE /files/{filename}", handler.DeleteFile)

	server := http.Server{
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 20 * time.Second,
		IdleTimeout:  30 * time.Second,
		Addr:         ":" + cfg.Port,
		Handler:      mux,
	}

	slog.Info("server start", "addr", server.Addr)

	err = server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		slog.Error("server fatal", "error", err)
	}
}
