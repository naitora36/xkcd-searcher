package rest

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"yadro.com/course/api/core"
)

const phraseKey = "phrase"

type PingResponse struct {
	Replies map[string]string `json:"replies"`
}
type NormResponse struct {
	Words []string `json:"words"`
	Total int      `json:"total"`
}

func NewPingHandler(log *slog.Logger, pingers map[string]core.Pinger) http.HandlerFunc {
	type pingResult struct {
		name   string
		status string
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var wg sync.WaitGroup
		results := make(chan pingResult, len(pingers))
		replies := make(map[string]string, len(pingers))

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		for name, pinger := range pingers {
			wg.Go(func() {
				err := pinger.Ping(ctx)
				if err != nil {
					log.Warn("ping service error", "service", name, "error", err)
					results <- pingResult{
						name:   name,
						status: "unavailable",
					}
					return
				}

				results <- pingResult{
					name:   name,
					status: "ok",
				}
			})
		}

		go func() {
			wg.Wait()
			close(results)
		}()

		for res := range results {
			replies[res.name] = res.status
		}

		sendJSON(w, log, PingResponse{Replies: replies})
	}
}

func NewWordsHandler(log *slog.Logger, normalizer core.Normalizer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		phrase := r.URL.Query().Get(phraseKey)

		words, err := normalizer.Norm(r.Context(), phrase)
		if err != nil {
			if errors.Is(err, core.ErrEmptyPhrase) {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if errors.Is(err, core.ErrResourceExhausted) {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			http.Error(w, "normalize phrase error", http.StatusInternalServerError)
			return
		}

		result := NormResponse{
			Words: words,
			Total: len(words),
		}

		sendJSON(w, log, result)
	}
}

func sendJSON(w http.ResponseWriter, log *slog.Logger, data any) {
	body, err := json.Marshal(data)
	if err != nil {
		log.Error("failed to encode response", "error", err)
		http.Error(w, core.ErrInternal.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	_, err = w.Write(body)
	if err != nil {
		log.Error("write response error", "error", err)
		return
	}
}
