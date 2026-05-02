package handler

import (
	"fmt"
	"log/slog"
	"net/http"
)

func Hello(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "empty name", http.StatusBadRequest)
		return
	}
	_, err := fmt.Fprintf(w, "Hello, %v!\n", name)
	if err != nil {
		slog.Error("failed to write response", "error", err)
	}
}

func Ping(w http.ResponseWriter, r *http.Request) {
	_, err := fmt.Fprintln(w, "pong")
	if err != nil {
		slog.Error("failed to write response", "error", err)
	}
}
