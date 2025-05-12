/*
Copyright 2025 The Nuclio Authors.

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

package logProxy

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"

	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/elastic/go-elasticsearch/v9"
	"github.com/nuclio/errors"
)

type ElasticLogProxy struct {
	client *elasticsearch.TypedClient

	index            string
	customQueryParam string
}

func NewElasticLogProxy(config *platformconfig.ElasticSearchConfig) (*ElasticLogProxy, error) {
	esClient := &ElasticLogProxy{
		index:            config.Index,
		customQueryParam: config.CustomQueryParameter,
	}
	var err error

	tlsConfig := &tls.Config{
		// Set to true to skip TLS verification if SSLVerificationMode is "none"
		InsecureSkipVerify: config.SSLVerificationMode == "none",
	}
	if esClient.client, err = elasticsearch.NewTypedClient(elasticsearch.Config{
		Addresses: []string{config.URL},
		Password:  config.Password,
		Username:  config.Username,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}); err != nil {
		return nil, errors.Wrap(err, "Failed to create elasticsearch client")
	}
	return esClient, err
}

func (e *ElasticLogProxy) ProxyFunctionLogs(ctx context.Context, options *platform.ProxyFunctionLogsOptions) (io.ReadCloser, error) {
	return nil, nil
}

func (e *ElasticLogProxy) GetFunctionReplicas(ctx context.Context, options *GetFunctionReplicaOptions) ([]string, error) {
	return nil, nil
}
