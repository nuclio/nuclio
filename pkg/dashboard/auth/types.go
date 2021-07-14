package auth

import (
	"net/http"
)

type Kind string

const (
	KindNop     = "nop"
	KindIguazio = "iguazio"
)

type Auth interface {
	Authenticate(request *http.Request) error
}
