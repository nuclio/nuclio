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
	"fmt"
	"net/http"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/nuclio/errors"
)

// AuthenticationMode is the authentication mode for API gateways and ingress resources.
type AuthenticationMode string

const (
	AuthenticationModeNone      AuthenticationMode = "none"
	AuthenticationModeBasicAuth AuthenticationMode = "basicAuth"
	AuthenticationModeAccessKey AuthenticationMode = "accessKey"
	AuthenticationModeOauth2    AuthenticationMode = "oauth2"
	AuthenticationModeIguazio   AuthenticationMode = "iguazio"
	AuthenticationModeAPI       AuthenticationMode = "api"
	AuthenticationModeBrowser   AuthenticationMode = "browser"

	// AttributeAuthenticationMode is the key for the authentication mode in HTTP trigger attributes.
	AttributeAuthenticationMode = "authenticationMode"
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

const (
	// FunctionContainerHTTPLoopbackPort is the port the processor listens on when the auth-proxy sidecar
	// is injected in front of it: only reachable from within the pod (127.0.0.1), never exposed by the Service.
	FunctionContainerHTTPLoopbackPort = 6080

	// AuthProxySidecarListenPortRangeStart and AuthProxySidecarListenPortRangeEnd bound the band the
	// auth-proxy listens on to front user sidecar ports. A user sidecar keeps binding its own port, so the
	// proxy cannot reuse it; instead the Service's targetPort is repointed at a port from this band.
	AuthProxySidecarListenPortRangeStart = 6081
	AuthProxySidecarListenPortRangeEnd   = 6199

	// AuthProxySidecarContainerName is the reserved name of the platform-injected auth-proxy sidecar.
	// User-defined sidecars may not use this name.
	AuthProxySidecarContainerName = "auth-proxy"
)

// AuthProxySidecarPortName returns the container-port name the auth-proxy sidecar binds for a listen port.
func AuthProxySidecarPortName(listenPort int) string {
	return fmt.Sprintf("authproxy-%d", listenPort)
}

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

// Function level authentication configuration, resolved from the HTTP trigger's attributes.

// FunctionAuthConfig is the authentication configuration resolved for a single function.
type FunctionAuthConfig struct {
	Mode              AuthenticationMode
	BasicAuthUsername string
	BasicAuthPassword string // plaintext input; cleared after hashing in reverseProxy mode

	// BasicAuthPasswordHash is the bcrypt hash of BasicAuthPassword. Set by NewReverseProxyAuthenticator
	// so the plaintext is never held in memory for the pod's lifetime.
	BasicAuthPasswordHash []byte
}

// Validate checks mode-specific required fields.
func (c FunctionAuthConfig) Validate() error {
	if c.Mode == AuthenticationModeBasicAuth {
		if c.BasicAuthUsername == "" {
			return errors.New("Basic-auth username must be provided")
		}
		if c.BasicAuthPassword == "" {
			return errors.New("Basic-auth password must be provided")
		}
	}
	return nil
}

// httpTriggerAuthAttrs is the decode target for the HTTP trigger's auth-related attributes.
type httpTriggerAuthAttrs struct {
	AuthenticationMode string `mapstructure:"authenticationMode"`
	Authentication     *struct {
		BasicAuth *struct {
			Username string `mapstructure:"username"`
			Password string `mapstructure:"password"`
		} `mapstructure:"basicAuth"`
	} `mapstructure:"authentication"`
}

// FunctionAuthConfigFromAttributes decodes authenticationMode + authentication.basicAuth from an HTTP
// trigger's free-form attributes.
func FunctionAuthConfigFromAttributes(attributes map[string]interface{}, defaultMode AuthenticationMode) (FunctionAuthConfig, error) {
	var decoded httpTriggerAuthAttrs
	if err := mapstructure.Decode(attributes, &decoded); err != nil {
		return FunctionAuthConfig{}, errors.Wrap(err, "Failed to decode HTTP trigger attributes")
	}

	mode := AuthenticationMode(decoded.AuthenticationMode)
	if mode == "" {
		mode = defaultMode
	}

	authConfig := FunctionAuthConfig{Mode: mode}
	switch mode {
	case AuthenticationModeNone, AuthenticationModeAPI, AuthenticationModeBrowser:
		// known modes that don't require any additional config
	case AuthenticationModeBasicAuth:
		if decoded.Authentication != nil && decoded.Authentication.BasicAuth != nil {
			authConfig.BasicAuthUsername = decoded.Authentication.BasicAuth.Username
			authConfig.BasicAuthPassword = decoded.Authentication.BasicAuth.Password
		}
	default:
		return FunctionAuthConfig{}, errors.Errorf("Unknown authentication mode: %s", decoded.AuthenticationMode)
	}

	if err := authConfig.Validate(); err != nil {
		return FunctionAuthConfig{}, err
	}
	return authConfig, nil
}

// FunctionLevelAuthenticationModes are the authentication modes valid for an HTTP trigger's
// authenticationMode attribute: mode-based API authentication, browser-redirect authentication, and
// static basic-auth credentials. AuthenticationModeNone (no additional authentication) is deliberately
// excluded - it means the function-level auth-proxy gate itself does not apply.
var FunctionLevelAuthenticationModes = map[AuthenticationMode]struct{}{
	AuthenticationModeAPI:       {},
	AuthenticationModeBrowser:   {},
	AuthenticationModeBasicAuth: {},
}

// IsFunctionLevelAuthenticationMode reports whether mode is one of FunctionLevelAuthenticationModes.
func IsFunctionLevelAuthenticationMode(mode string) bool {
	_, ok := FunctionLevelAuthenticationModes[AuthenticationMode(mode)]
	return ok
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
