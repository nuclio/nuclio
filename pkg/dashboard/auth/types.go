package auth

import (
	"net/http"
)

type Mode string

const (
	ModeNop     = "nop"
	ModeIguazio = "iguazio"
)

type Auth interface {
	Authenticate(request *http.Request) error
}
