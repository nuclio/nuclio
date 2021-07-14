package iguazio

import (
	"net/http"

	"github.com/nuclio/nuclio/pkg/dashboard/auth"

	"github.com/nuclio/logger"
)

// AuthenticationMiddleware implements Iguazio session key authentication
func AuthenticationMiddleware(logger logger.Logger, options *auth.Options) func(next http.Handler) http.Handler {
	authInstance := NewAuth(logger)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := authInstance.Authenticate(r, options); err != nil {
				iguazioAuthenticationFailed(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func iguazioAuthenticationFailed(w http.ResponseWriter) {
	w.WriteHeader(http.StatusUnauthorized)
}
