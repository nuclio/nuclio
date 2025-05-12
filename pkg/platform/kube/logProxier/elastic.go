package logProxier

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"

	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/nuclio/errors"
)

type ElasticLogProxier struct {
	client *elasticsearch.TypedClient

	index            string
	customQueryParam string
}

func NewElasticLogProxier(config *platformconfig.ElasticSearchConfig) (*ElasticLogProxier, error) {
	esClient := &ElasticLogProxier{
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

func (e *ElasticLogProxier) ProxyFunctionLogs(ctx context.Context, options *platform.ProxyFunctionLogsOptions) (io.ReadCloser, error) {
	return nil, nil
}

func (e *ElasticLogProxier) GetFunctionReplicas(ctx context.Context, options *GetFunctionReplicaOptions) ([]string, error) {
	return nil, nil
}
