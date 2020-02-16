package v3iohttp

import (
	"encoding/base64"
	"fmt"

	"github.com/v3io/v3io-go/pkg/dataplane"

	"github.com/nuclio/logger"
)

type session struct {
	logger              logger.Logger
	context             *context
	url                 string
	authenticationToken string
	accessKey           string
}

func newSession(parentLogger logger.Logger,
	context *context,
	url string,
	username string,
	password string,
	accessKey string) (v3io.Session, error) {

	authenticationToken := ""
	if username != "" && password != "" && accessKey == "" {
		authenticationToken = GenerateAuthenticationToken(username, password)
	}

	return &session{
		logger:              parentLogger.GetChild("session"),
		context:             context,
		url:                 url,
		authenticationToken: authenticationToken,
		accessKey:           accessKey,
	}, nil
}

// NewContainer creates a container
func (s *session) NewContainer(newContainerInput *v3io.NewContainerInput) (v3io.Container, error) {
	return newContainer(s.logger, s, newContainerInput.ContainerName)
}

func GenerateAuthenticationToken(username string, password string) string {

	// generate token for basic authentication
	usernameAndPassword := fmt.Sprintf("%s:%s", username, password)
	encodedUsernameAndPassword := base64.StdEncoding.EncodeToString([]byte(usernameAndPassword))

	return "Basic " + encodedUsernameAndPassword
}
