package middleware

import (
	"net/http"
)

func Concurrency(next http.HandlerFunc, limit int) http.HandlerFunc {
	sema := make(chan struct{}, limit)
	return func(w http.ResponseWriter, r *http.Request) {
		select {
		case sema <- struct{}{}:
		case <-r.Context().Done():
			return
		default:
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}

		defer func() {
			<-sema
		}()

		next(w, r)
	}
}
