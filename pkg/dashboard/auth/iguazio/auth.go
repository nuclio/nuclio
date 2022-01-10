package iguazio

import (
	"bytes"
	"context"
	"crypto/tls"
	"net/http"
	"strings"

	"github.com/nuclio/nuclio/pkg/dashboard/auth"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	"k8s.io/apimachinery/pkg/util/cache"
)

const IguzioUsernameLabel string = "iguazio.com/username"

type Auth struct {
	logger     logger.Logger
	config     *auth.Config
	httpClient *http.Client
	cache      *cache.LRUExpireCache
}

func NewAuth(logger logger.Logger, config *auth.Config) auth.Auth {
	return &Auth{
		logger: logger.GetChild("iguazio-auth"),
		config: config,
		httpClient: &http.Client{
			Timeout: config.Iguazio.Timeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
		cache: cache.NewLRUExpireCache(config.Iguazio.CacheSize),
	}
}

// Authenticate will ask IguazioConfig session verification endpoint to verify the request session
// and enrich with session metadata
func (a *Auth) Authenticate(request *http.Request) (auth.Session, error) {
	authorization := request.Header.Get("authorization")
	cookie := request.Header.Get("cookie")
	cacheKey := authorization + cookie

	if cacheKey == "" {
		return nil, nuclio.NewErrForbidden("Authentication headers are missing")
	}

	// try resolve from cache
	if cacheData, found := a.cache.Get(cacheKey); found {
		return cacheData.(*auth.IguazioSession), nil
	}

	authHeaders := map[string]string{
		"authorization": authorization,
		"cookie":        cookie,
	}

	response, err := a.performHTTPRequest(http.MethodPost,
		a.config.Iguazio.VerificationURL,
		nil,
		map[string]string{
			"authorization": authorization,
			"cookie":        cookie,
		})
	if err != nil {
		a.logger.WarnWith("Failed to perform http authentication request",
			"err", err,
		)
		return nil, errors.Wrap(err, "Failed to perform http POST request")
	}

	// auth failed
	if response.StatusCode == http.StatusUnauthorized {
		a.logger.WarnWith("Authentication failed",
			"authorizationHeaderLength", len(authHeaders["authorization"]),
			"cookieHeaderLength", len(authHeaders["cookie"]),
		)
		return nil, nuclio.NewErrUnauthorized("Authentication failed")
	}

	// not within range of 200
	if !(response.StatusCode >= http.StatusOK && response.StatusCode < 300) {
		a.logger.WarnWith("Unexpected authentication status code",
			"authorizationHeaderLength", len(authHeaders["authorization"]),
			"cookieHeaderLength", len(authHeaders["cookie"]),
			"statusCode", response.StatusCode,
		)
		return nil, nuclio.NewErrUnauthorized("Authentication failed")
	}

	authInfo := &auth.IguazioSession{
		Username:   response.Header.Get("x-remote-user"),
		SessionKey: response.Header.Get("x-v3io-session-key"),
		UserID:     response.Header.Get("x-user-id"),
	}

	for _, groupID := range response.Header.Values("x-user-group-ids") {
		if groupID != "" {
			authInfo.GroupIDs = append(authInfo.GroupIDs, strings.Split(groupID, ",")...)
		}
	}

	a.cache.Add(authorization+cookie, authInfo, a.config.Iguazio.CacheExpirationTimeout)
	a.logger.InfoWith("Authentication succeeded", "username", authInfo.GetUsername())
	return authInfo, nil
}

// Middleware will authenticate the incoming request and store the session within the request context
func (a *Auth) Middleware() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, err := a.Authenticate(r)
			ctx := r.Context()
			if err != nil {
				a.logger.WarnWithCtx(ctx, "Authentication failed",
					"headers", r.Header)
				a.iguazioAuthenticationFailed(w)
				return
			}
			a.logger.DebugWithCtx(ctx, "Successfully authenticated incoming request",
				"sessionUsername", session.GetUsername())
			enrichedCtx := context.WithValue(ctx, auth.IguazioContextKey, session)
			next.ServeHTTP(w, r.WithContext(enrichedCtx))
		})
	}
}

func (a *Auth) Kind() auth.Kind {
	return a.config.Kind
}

func (a *Auth) iguazioAuthenticationFailed(w http.ResponseWriter) {
	w.WriteHeader(http.StatusUnauthorized)
}

func (a *Auth) performHTTPRequest(method string,
	url string,
	body []byte,
	headers map[string]string) (*http.Response, error) {

	// create request
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create http request")
	}

	// attach headers
	for headerKey, headerValue := range headers {
		req.Header.Set(headerKey, headerValue)
	}

	// fire request
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to send HTTP request")
	}

	return resp, nil
}
