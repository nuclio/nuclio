package iguazio

import (
	"context"
	"net/http"

	"github.com/nuclio/nuclio/pkg/dashboard/auth/config"

	"github.com/nuclio/logger"
)

// AuthenticationMiddleware implements Iguazio session key authentication
func AuthenticationMiddleware(logger logger.Logger, options *config.Options) func(next http.Handler) http.Handler {
	authInstance := NewAuth(logger, options)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := authInstance.Authenticate(r); err != nil {
				iguazioAuthenticationFailed(w)
				return
			}
			ctx := context.WithValue(r.Context(), config.IguazioContextKey, authInstance)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func iguazioAuthenticationFailed(w http.ResponseWriter) {
	w.WriteHeader(http.StatusUnauthorized)
}
