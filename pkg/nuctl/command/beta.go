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

package command

import (
	"context"

	"github.com/nuclio/nuclio/pkg/nuctl/client"

	"github.com/nuclio/errors"
	"github.com/spf13/cobra"
)

const DefaultConcurrency = 10

// betaCommandeer is the BETA version of nuctl as an API client
type betaCommandeer struct {
	cmd            *cobra.Command
	rootCommandeer *RootCommandeer
	apiClient      client.APIClient
	concurrency    int
	apiURL         string
	username       string
	accessKey      string
	requestTimeout string
	skipTLSVerify  bool
}

func newBetaCommandeer(ctx context.Context, rootCommandeer *RootCommandeer) *betaCommandeer {
	commandeer := &betaCommandeer{
		rootCommandeer: rootCommandeer,
	}

	cmd := &cobra.Command{
		Use:   "beta",
		Short: "A beta version of nuctl as a Nuclio api cli client",
	}

	cmd.PersistentFlags().StringVar(&commandeer.apiURL, "api-url", "", "URL of the nuclio API (e.g. https://nuclio.io:8070)")
	cmd.PersistentFlags().StringVar(&commandeer.username, "username", "", "Username of a user with permissions to the nuclio API")
	cmd.PersistentFlags().StringVar(&commandeer.accessKey, "access-key", "", "Access Key of a user with permissions to the nuclio API")
	cmd.PersistentFlags().StringVar(&commandeer.requestTimeout, "request-timeout", "60s", "Request timeout")
	cmd.PersistentFlags().BoolVar(&commandeer.skipTLSVerify, "skip-tls-verify", false, "Skip TLS verification")
	cmd.PersistentFlags().IntVar(&commandeer.concurrency, "concurrency", DefaultConcurrency, "Max number of parallel patches")

	cmd.MarkPersistentFlagRequired("api-url") // nolint: errcheck

	cmd.AddCommand(
		newDeployCommandeer(ctx, rootCommandeer, commandeer).cmd,
	)

	commandeer.cmd = cmd

	return commandeer
}

func (b *betaCommandeer) initialize() error {
	var err error

	// initialize root
	if err := b.rootCommandeer.initialize(); err != nil {
		return errors.Wrap(err, "Failed to initialize root")
	}

	b.apiClient, err = client.NewNuclioAPIClient(b.rootCommandeer.loggerInstance,
		b.apiURL,
		b.requestTimeout,
		b.username,
		b.accessKey,
		b.skipTLSVerify)
	if err != nil {
		return errors.Wrap(err, "Failed to create API client")
	}

	return nil
}
