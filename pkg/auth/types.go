/*
Copyright 2017 The Nuclio Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
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
	Timeout                       time.Duration
	VerificationURL               string
	VerificationDataEnrichmentURL string
	CacheSize                     int
	CacheExpirationTimeout        time.Duration
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

type Options struct {
	EnrichDataPlane bool
}

type Session interface {
	GetUsername() string
	GetPassword() string
	GetUserID() string
	GetGroupIDs() []string
	CompileAuthorizationBasic() string
}

type Auth interface {
	Authenticate(request *http.Request, options *Options) (Session, error)
	Middleware(options *Options) func(http.Handler) http.Handler
	Kind() Kind
}
