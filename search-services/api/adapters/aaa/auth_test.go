package aaa

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"yadro.com/course/api/core"
)

func TestNew(t *testing.T) {
	origUser := os.Getenv("ADMIN_USER")
	origPass := os.Getenv("ADMIN_PASSWORD")
	t.Cleanup(func() {
		err := os.Setenv("ADMIN_USER", origUser)
		require.NoError(t, err)

		err = os.Setenv("ADMIN_PASSWORD", origPass)
		require.NoError(t, err)
	})

	logger := slog.New(slog.DiscardHandler)

	t.Run("success", func(t *testing.T) {
		err := os.Setenv("ADMIN_USER", "admin")
		require.NoError(t, err)

		err = os.Setenv("ADMIN_PASSWORD", "secret")
		require.NoError(t, err)

		a, err := New(time.Hour, logger)
		require.NoError(t, err)
		require.Equal(t, "secret", a.users["admin"])
		require.Equal(t, time.Hour, a.tokenTTL)
	})

	t.Run("missing user", func(t *testing.T) {
		err := os.Unsetenv("ADMIN_USER")
		require.NoError(t, err)

		err = os.Setenv("ADMIN_PASSWORD", "secret")
		require.NoError(t, err)

		_, err = New(time.Hour, logger)
		require.Error(t, err)
		require.Contains(t, err.Error(), "could not get admin user")
	})

	t.Run("missing password", func(t *testing.T) {
		err := os.Setenv("ADMIN_USER", "admin")
		require.NoError(t, err)

		err = os.Unsetenv("ADMIN_PASSWORD")
		require.NoError(t, err)

		_, err = New(time.Hour, logger)
		require.Error(t, err)
		require.Contains(t, err.Error(), "could not get admin password")
	})
}

func TestAAA_Login(t *testing.T) {
	a := AAA{
		users:    map[string]string{"admin": "password123"},
		tokenTTL: time.Hour,
		log:      slog.New(slog.DiscardHandler),
	}

	tests := []struct {
		name        string
		user        string
		password    string
		expectedErr error
	}{
		{"correct credentials", "admin", "password123", nil},
		{"wrong password", "admin", "wrong", core.ErrUnauthorized},
		{"wrong user", "hacker", "password123", core.ErrUnauthorized},
		{"empty credentials", "", "", core.ErrUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := a.Login(tt.user, tt.password)

			if tt.expectedErr == nil {
				require.NoError(t, err)
				require.NotEmpty(t, token)
			} else {
				require.ErrorIs(t, err, tt.expectedErr)
				require.Empty(t, token)
			}
		})
	}
}

func TestAAA_Verify(t *testing.T) {
	a := AAA{log: slog.New(slog.DiscardHandler)}

	forgeToken := func(subject string, expiresAt time.Time, signKey string) string {
		claim := MyClaim{
			User: "admin",
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   subject,
				ExpiresAt: jwt.NewNumericDate(expiresAt),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claim)
		str, _ := token.SignedString([]byte(signKey))
		return str
	}

	validToken := forgeToken(adminRole, time.Now().Add(time.Hour), secretKey)
	expiredToken := forgeToken(adminRole, time.Now().Add(-time.Hour), secretKey)
	wrongSubjectToken := forgeToken("guest", time.Now().Add(time.Hour), secretKey)
	wrongSignToken := forgeToken(adminRole, time.Now().Add(time.Hour), "hacker_key")

	tests := []struct {
		name        string
		token       string
		expectedErr error
	}{
		{"valid token", validToken, nil},
		{"expired token", expiredToken, core.ErrUnauthorized},
		{"wrong subject", wrongSubjectToken, core.ErrUnauthorized},
		{"invalid signature", wrongSignToken, core.ErrUnauthorized},
		{"garbage string", "not.a.jwt.token", core.ErrUnauthorized},
		{"empty token", "", core.ErrUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := a.Verify(tt.token)

			if tt.expectedErr == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tt.expectedErr)
			}
		})
	}
}
