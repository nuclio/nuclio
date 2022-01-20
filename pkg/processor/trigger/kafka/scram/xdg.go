package scram

import (
	"crypto/sha256"
	"crypto/sha512"

	"github.com/Shopify/sarama"
	"github.com/nuclio/errors"
	"github.com/xdg-go/scram"
)

type Client struct {
	*scram.Client
	*scram.ClientConversation
	scram.HashGeneratorFcn
}

func NewClient(saslMechanism sarama.SASLMechanism) sarama.SCRAMClient {
	var HashGeneratorFcn scram.HashGeneratorFcn

	switch saslMechanism {
	case sarama.SASLTypeSCRAMSHA256:
		HashGeneratorFcn = sha256.New
	case sarama.SASLTypeSCRAMSHA512:
		HashGeneratorFcn = sha512.New
	default:
		HashGeneratorFcn = sha256.New
	}
	return &Client{
		HashGeneratorFcn: HashGeneratorFcn,
	}

}

func (sc *Client) Begin(userName, password, authzID string) (err error) {
	sc.Client, err = sc.HashGeneratorFcn.NewClient(userName, password, authzID)
	if err != nil {
		return errors.Wrap(err, "Failed to create new client")
	}
	sc.ClientConversation = sc.Client.NewConversation()
	return nil
}

func (sc *Client) Step(challenge string) (string, error) {
	response, err := sc.ClientConversation.Step(challenge)
	if err != nil {
		return "", errors.Wrap(err, "Failed to step")
	}
	return response, nil
}

func (sc *Client) Done() bool {
	return sc.ClientConversation.Done()
}
