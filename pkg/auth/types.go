/*
Copyright 2023 The Nuclio Authors.

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

// AuthenticationMode is the authentication mode for API gateways and ingress resources.
type AuthenticationMode string

const (
	AuthenticationModeNone      AuthenticationMode = "none"
	AuthenticationModeBasicAuth AuthenticationMode = "basicAuth"
	AuthenticationModeAccessKey AuthenticationMode = "accessKey"
	AuthenticationModeOauth2    AuthenticationMode = "oauth2"
	AuthenticationModeIguazio   AuthenticationMode = "iguazio"
)

type Kind string

const (
	KindNop       = "nop"
	KindIguazio   = "iguazio"
	KindIguazioV4 = "iguazio-v4"
)

// ProxyMode selects how the auth-proxy operates.
type ProxyMode string

const (
	// ProxyModeReverseProxy fronts a function: authenticates each request, forwards allowed ones to the processor.
	ProxyModeReverseProxy ProxyMode = "reverseProxy"

	// ProxyModeAuthOnly serves only the /auth endpoint (called by the DLX); does no forwarding.
	ProxyModeAuthOnly ProxyMode = "authOnly"
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
	case KindIguazio, KindIguazioV4:
		return IguazioContextKey
	default:
		return NopContextKey
	}
}

type IguazioConfig struct {
	Timeout                time.Duration
	VerificationURL        string
	VerificationMethod     string
	CacheSize              int
	CacheExpirationTimeout time.Duration
	SkipTLSVerification    bool

	// igz < v4
	VerificationDataEnrichmentURL string
}

type Config struct {
	Kind    Kind
	Iguazio *IguazioConfig
}

func NewConfig(kind Kind) *Config {
	config := &Config{
		Kind: kind,
	}
	skipTLSVerification := false
	if kind == KindIguazio || kind == KindIguazioV4 {
		skipTLSVerification = true
		config.Iguazio = &IguazioConfig{
			CacheSize:              100,
			Timeout:                30 * time.Second,
			CacheExpirationTimeout: 30 * time.Second,
			SkipTLSVerification:    skipTLSVerification,
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
	CompileAuthorizationHeader() string
	GetUserLabels() map[string]string
}

type Auth interface {
	Authenticate(request *http.Request, options *Options) (Session, error)
	Middleware(options *Options) func(http.Handler) http.Handler
	Kind() Kind
}
