package iguazio

import (
	"net/http"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/dashboard/auth"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type Auth struct {
	logger logger.Logger

	username   string
	sessionKey string
	uid        []string
	gid        []string
}

func NewAuth(logger logger.Logger) auth.Auth {
	return &Auth{
		logger: logger.GetChild("iguazio-auth"),
	}
}

// Authenticate will ask Iguazio session verification endpoint to verify the request session
// and enrich with session metadata
func (a *Auth) Authenticate(request *http.Request, options *auth.Options) error {
	headers := map[string]string{
		"authorization": request.Header.Get("authorization"),
		"cookie":        request.Header.Get("cookie"),
	}

	timeout := 60 * time.Second
	if options.Timeout != nil {
		timeout = *options.Timeout
	}

	_, response, err := common.SendHTTPRequest(http.MethodPost,
		*options.VerificationURL,
		nil,
		headers,
		nil,
		http.StatusOK,
		true,
		timeout)
	if err != nil {
		return errors.Wrap(err, "Failed to authorize request")
	}

	a.username = response.Header.Get("x-remote-user")
	a.sessionKey = response.Header.Get("x-v3io-session-key")
	a.uid = []string{response.Header.Get("x-user-id")}
	gid := response.Header.Get("x-user-group-ids")
	if gid != "" {
		a.gid = strings.Split(gid, ",")
	}
	return nil
}
