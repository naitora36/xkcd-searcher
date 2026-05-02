package rest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
type UpdateStatsDto struct {
	WordsTotal    int `json:"words_total"`
	WordsUnique   int `json:"words_unique"`
	ComicsFetched int `json:"comics_fetched"`
	ComicsTotal   int `json:"comics_total"`
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

func NewUpdateHandler(log *slog.Logger, updater core.Updater) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		err := updater.Update(ctx)
		if err == nil {
			w.WriteHeader(http.StatusOK)

			_, err = fmt.Fprintln(w, "the update was successful")
			if err != nil {
				log.Error("sending response error", "error", err)
				return
			}
			return
		}

		if errors.Is(err, core.ErrAlreadyRunning) {
			w.WriteHeader(http.StatusAccepted)
			_, err := fmt.Fprintln(w, "update request has already been sent")
			if err != nil {
				log.Error("sending response error", "error", err)
				return
			}
			return
		}

		log.Error("update failed", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func NewUpdateStatsHandler(log *slog.Logger, updater core.Updater) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		stats, err := updater.Stats(ctx)
		if err != nil {
			log.Error("get stats error", "error", err.Error())
			http.Error(w, core.ErrInternal.Error(), http.StatusInternalServerError)
			return
		}

		res := UpdateStatsDto{
			WordsTotal:    stats.WordsTotal,
			WordsUnique:   stats.WordsUnique,
			ComicsFetched: stats.ComicsFetched,
			ComicsTotal:   stats.ComicsTotal,
		}

		sendJSON(w, log, res)
	}
}

func NewUpdateStatusHandler(log *slog.Logger, updater core.Updater) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		status, err := updater.Status(ctx)
		if err != nil {
			log.Error("fatal error when get status", "error", err)
			http.Error(w, core.ErrInternal.Error(), http.StatusInternalServerError)
			return
		}
		response := struct {
			Status string `json:"status"`
		}{
			Status: string(status),
		}

		sendJSON(w, log, response)
	}
}

func NewDropHandler(log *slog.Logger, updater core.Updater) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		err := updater.Drop(ctx)
		if err == nil {
			w.WriteHeader(http.StatusOK)

			_, err := fmt.Fprintln(w, "the drop table was successful")
			if err != nil {
				log.Error("sending response error", "error", err)
				return
			}
			return
		}

		if errors.Is(err, core.ErrAlreadyRunning) {
			http.Error(w, "service is busy with another operation", http.StatusConflict)
			return
		}
		log.Error("fatal error when drop table", "error", err)
		http.Error(w, core.ErrInternal.Error(), http.StatusInternalServerError)
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
