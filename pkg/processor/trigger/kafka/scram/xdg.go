/*
Copyright 2017 The Nuclio Authors.

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
