package middleware

import (
	"context"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

func Rate(next http.HandlerFunc, rps int, timeoutQueue time.Duration) http.HandlerFunc {
	burst := rps - ((rps * 25) / 100)

	limiter := rate.NewLimiter(rate.Limit(rps), burst)
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeoutQueue)
		defer cancel()

		if err := limiter.Wait(ctx); err != nil {
			if r.Context().Err() != nil {
				return
			}

			http.Error(w, "too many requests, server busy", http.StatusTooManyRequests)
			return
		}

		next(w, r)
	}
}
