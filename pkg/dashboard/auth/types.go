package auth

import (
	"net/http"
	"time"
)

type Kind string

const (
	KindNop     = "nop"
	KindIguazio = "iguazio"
)

type SessionContextKey string

const (
	IguazioContextKey     SessionContextKey = "IguazioSession"
	NopContextKey         SessionContextKey = "NopSession"
	AuthSessionContextKey SessionContextKey = "AuthSession"
)

func ContextKeyByKind(kind Kind) SessionContextKey {
	switch kind {
	case KindNop:
		return NopContextKey
	case KindIguazio:
		return IguazioContextKey
	default:
		return NopContextKey
	}
}

type IguazioConfig struct {
	Timeout                time.Duration
	VerificationURL        string
	CacheSize              int
	CacheExpirationTimeout time.Duration
}

type Config struct {
	Kind    Kind
	Iguazio *IguazioConfig
}

func NewConfig(kind Kind) *Config {
	config := &Config{
		Kind: kind,
	}
	if kind == KindIguazio {
		config.Iguazio = &IguazioConfig{
			CacheSize:              100,
			Timeout:                30 * time.Second,
			CacheExpirationTimeout: 30 * time.Second,
		}
	}
	return config
}

type Session interface {
	GetUsername() string
	GetPassword() string
	GetUserID() string
	GetGroupIDs() []string
	CompileAuthorizationBasic() string
}

type Auth interface {
	Authenticate(request *http.Request) (Session, error)
	Middleware() func(http.Handler) http.Handler
	Kind() Kind
}
