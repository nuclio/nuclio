/*
Copyright 2026 The Nuclio Authors.

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

package serviceaccounttoken

import (
	"fmt"
	"maps"

	"github.com/nuclio/nuclio/pkg/auth/utils"
	"github.com/nuclio/nuclio/pkg/common/headers"
	nuctlcommon "github.com/nuclio/nuclio/pkg/nuctl/command/common"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
)

type Client struct {
	tokenPath                    string
	tokenExpirationSeconds       int
	tokenExpirationBufferSeconds float64

	tokenCache string
}

func NewClient(serviceAccountConfig *platformconfig.ServiceAccountConfig) (*Client, error) {
	tokenPath := DefaultTokenPath
	tokenExpirationSeconds := DefaultTokenExpirationSeconds
	tokenRefreshRatio := DefaultTokenRefreshRatio

	if serviceAccountConfig != nil {
		if serviceAccountConfig.TokenPath != nil && *serviceAccountConfig.TokenPath != "" {
			tokenPath = *serviceAccountConfig.TokenPath
		}
		if serviceAccountConfig.TokenExpirationSeconds != nil {
			tokenExpirationSeconds = *serviceAccountConfig.TokenExpirationSeconds
		}
		if serviceAccountConfig.TokenRefreshRatio != nil {
			tokenRefreshRatio = *serviceAccountConfig.TokenRefreshRatio
		}
	}

	return &Client{
		tokenPath:                    tokenPath,
		tokenExpirationSeconds:       tokenExpirationSeconds,
		tokenExpirationBufferSeconds: float64(tokenExpirationSeconds) * (1 - tokenRefreshRatio),
	}, nil
}

// EscalateAuthHeaders adds the service account authentication headers to the provided headers map
func (c *Client) EscalateAuthHeaders(headers map[string]string) error {
	authHeaders, err := c.AuthHeaders()
	if err != nil {
		return errors.Wrap(err, "Failed to get service account auth headers")
	}

	// add service account auth headers
	for k, v := range authHeaders {
		headers[k] = v
	}

	return nil
}

// AuthHeaders returns the authentication headers for the service account
func (c *Client) AuthHeaders() (map[string]string, error) {
	token, err := c.GetSAToken()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get service account token")
	}

	authHeaders := maps.Clone(ServiceAccountAuthenticationHeader)
	authHeaders[headers.AuthorizationHeader] = fmt.Sprintf("Bearer %s", token)

	return authHeaders, nil
}

// GetSAToken reads the service account token from the token file. It caches the token and refreshes it
// if it's close to expiration.
func (c *Client) GetSAToken() (string, error) {
	if c.tokenCache != "" {
		remaining, err := utils.TimeUntilExpiration(c.tokenCache)
		if err == nil && remaining.Seconds() > c.tokenExpirationBufferSeconds {
			return c.tokenCache, nil
		}
	}

	tokenFile, err := nuctlcommon.OpenFile(c.tokenPath)
	if err != nil {
		return "", errors.Wrapf(err, "Failed to open token file: %s", c.tokenPath)
	}
	defer tokenFile.Close()

	tokenBytes, err := nuctlcommon.ReadFromInOrStdin(tokenFile)
	if err != nil {
		return "", errors.Wrap(err, "Failed to read token file")
	}

	c.tokenCache = string(tokenBytes)
	return c.tokenCache, nil
}
