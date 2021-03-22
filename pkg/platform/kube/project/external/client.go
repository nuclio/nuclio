package external

import (
	"github.com/nuclio/nuclio/pkg/platform/kube/project"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type Client struct {
	*project.AbstractClient
}

func NewClient(parentLogger logger.Logger) (*Client, error) {
	client := Client{}

	abstractClient, err := project.NewAbstractClient(parentLogger)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create abstract client")
	}

	client.Client = abstractClient

	return &client, nil
}
