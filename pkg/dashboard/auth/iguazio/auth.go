package iguazio

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/nuclio/nuclio/pkg/dashboard/auth"
	authconfig "github.com/nuclio/nuclio/pkg/dashboard/auth/config"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type Auth struct {
	logger     logger.Logger
	options    *authconfig.Options
	httpClient *http.Client

	Username   string
	SessionKey string
	UserID     string
	GroupIDs   []string
}

func NewAuth(logger logger.Logger, options *authconfig.Options) auth.Auth {
	timeout := 30 * time.Second
	if options.Iguazio.Timeout != 0 {
		timeout = options.Iguazio.Timeout
	}
	return &Auth{
		logger:  logger.GetChild("iguazio-auth"),
		options: options,
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

// Authenticate will ask Iguazio session verification endpoint to verify the request session
// and enrich with session metadata
func (a *Auth) Authenticate(request *http.Request) error {
	headers := map[string]string{
		"authorization": request.Header.Get("authorization"),
		"cookie":        request.Header.Get("cookie"),
	}

	response, err := a.performHTTPRequest(http.MethodPost, a.options.Iguazio.VerificationURL, nil, headers)
	if err != nil {
		a.logger.WarnWith("Failed to perform http authentication request",
			"err", err,
		)
		return errors.Wrap(err, "Failed to perform http POST request")
	}

	// within range of 200
	if !(response.StatusCode >= http.StatusOK && response.StatusCode < 300) {
		a.logger.DebugWith("Invalid authentication status code response",
			"statusCode", response.StatusCode,
		)
		return errors.New(fmt.Sprintf("Unexpected authentication status code %d", response.StatusCode))
	}

	a.Username = response.Header.Get("x-remote-user")
	a.SessionKey = response.Header.Get("x-v3io-session-key")
	a.UserID = response.Header.Get("x-user-id")
	a.GroupIDs = response.Header.Values("x-user-group-ids")
	return nil
}

func (a *Auth) performHTTPRequest(method string,
	url string,
	body []byte,
	headers map[string]string) (*http.Response, error) {

	// create request object
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create http request")
	}

	// attach headers
	for headerKey, headerValue := range headers {
		req.Header.Set(headerKey, headerValue)
	}

	// perform the request
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to send HTTP request")
	}

	return resp, nil
}
