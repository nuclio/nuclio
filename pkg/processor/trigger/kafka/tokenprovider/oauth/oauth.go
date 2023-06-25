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

package oauth

import (
	"context"

	"github.com/Shopify/sarama"
	"github.com/nuclio/errors"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// TokenProvider encapsulates oauth2.TokenSource and returns sarama.AccessToken
type TokenProvider struct {
	tokenSource oauth2.TokenSource
}

func NewTokenProvider(ctx context.Context,
	clientID string,
	clientSecret string,
	tokenURL string,
	scopes []string) sarama.AccessTokenProvider {

	config := clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     tokenURL,
		Scopes:       scopes,
	}

	return &TokenProvider{
		tokenSource: config.TokenSource(ctx),
	}
}

// Token fetches token from the token source
func (t *TokenProvider) Token() (*sarama.AccessToken, error) {
	token, err := t.tokenSource.Token()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get token")
	}

	return &sarama.AccessToken{Token: token.AccessToken}, nil
}
