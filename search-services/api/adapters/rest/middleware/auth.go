package middleware

import (
	"net/http"
	"strings"
)

const (
	AuthorizationKey = "Authorization"
	TokenPrefix      = "Token "
)

type TokenVerifier interface {
	Verify(token string) error
}

func Auth(next http.HandlerFunc, verifier TokenVerifier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reqHeader := r.Header.Get(AuthorizationKey)

		tokenString := strings.TrimPrefix(reqHeader, TokenPrefix)

		err := verifier.Verify(tokenString)
		if err != nil {
			http.Error(w, "incorrect login or password", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}
