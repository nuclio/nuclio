package auth

import (
	"net/http"
	"time"
)

type Mode string

const (
	ModeNop     = "nop"
	ModeIguazio = "iguazio"
)

const DefaultMode = ModeNop

type Options struct {
	Mode Mode

	Timeout *time.Duration

	// iguazio
	VerificationURL *string
}

type Auth interface {
	Authenticate(request *http.Request, options *Options) error
}
