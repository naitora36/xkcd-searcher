package core

import "errors"

var (
	ErrBadArguments      = errors.New("arguments are not acceptable")
	ErrResourceExhausted = errors.New("phrase len more than limit")
	ErrAlreadyExists     = errors.New("resource or task already exists")
	ErrInternal          = errors.New("internal system error")
	ErrEmptyPhrase       = errors.New("phrase is empty")
	ErrDeadlineExceeded  = errors.New("deadline is exceeded")
	ErrCanceled          = errors.New("context is canceled")
	ErrAlreadyRunning    = errors.New("already run")
	ErrUnauthorized      = errors.New("user is unauthorized")
)
