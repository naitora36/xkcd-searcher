package aaa

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"yadro.com/course/api/core"
)

const (
	secretKey = "something secret here" // token sign key
	adminRole = "superuser"             // token subject
)

type MyClaim struct {
	User string `json:"user"`
	jwt.RegisteredClaims
}

// Authentication, Authorization, Accounting
type AAA struct {
	users    map[string]string
	tokenTTL time.Duration
	log      *slog.Logger
}

func New(tokenTTL time.Duration, log *slog.Logger) (AAA, error) {
	const adminUser = "ADMIN_USER"
	const adminPass = "ADMIN_PASSWORD"
	user, ok := os.LookupEnv(adminUser)
	if !ok {
		return AAA{}, fmt.Errorf("could not get admin user from enviroment")
	}
	password, ok := os.LookupEnv(adminPass)
	if !ok {
		return AAA{}, fmt.Errorf("could not get admin password from enviroment")
	}

	return AAA{
		users:    map[string]string{user: password},
		tokenTTL: tokenTTL,
		log:      log,
	}, nil
}

func (a AAA) Login(name, password string) (string, error) {
	passwordInStorage, ok := a.users[name]

	if password != passwordInStorage || !ok {
		return "", core.ErrUnauthorized
	}

	claim := MyClaim{
		User: name,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   adminRole,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(a.tokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claim)

	tokenString, err := token.SignedString([]byte(secretKey))
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func (a AAA) Verify(tokenString string) error {
	claims := &MyClaim{}

	token, err := jwt.ParseWithClaims(tokenString, claims,
		func(token *jwt.Token) (any, error) {
			return []byte(secretKey), nil
		},
		jwt.WithExpirationRequired(),
		jwt.WithValidMethods([]string{"HS256"}),
	)

	if claims.Subject != adminRole {
		return core.ErrUnauthorized
	}

	if err != nil || !token.Valid {
		a.log.Warn("invalid token attempt", "err", err)
		return core.ErrUnauthorized
	}

	return nil
}
