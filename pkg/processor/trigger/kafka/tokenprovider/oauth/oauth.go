package oauth

import (
	"context"

	"github.com/Shopify/sarama"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"

	"github.com/nuclio/errors"
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
