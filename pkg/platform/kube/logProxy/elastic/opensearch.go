package elastic

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"

	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/kube/logProxy"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
	"github.com/opensearch-project/opensearch-go"
)

type OpenSearchLogProxy struct {
	*BaseElasticLikeLogProxy
	client *opensearch.Client
}

func NewOpenSearchLogProxy(config *platformconfig.ElasticSearchConfig) (*OpenSearchLogProxy, error) {
	openSearchClient := &OpenSearchLogProxy{
		BaseElasticLikeLogProxy: &BaseElasticLikeLogProxy{
			index:            config.Index,
			customQueryParam: config.CustomQueryParameter,
		},
	}
	var err error

	tlsConfig := &tls.Config{
		// Set to true to skip TLS verification if SSLVerificationMode is "none"
		InsecureSkipVerify: config.SSLVerificationMode == "none",
	}
	if openSearchClient.client, err = opensearch.NewClient(opensearch.Config{
		Addresses: []string{config.URL},
		Password:  config.Password,
		Username:  config.Username,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}); err != nil {
		return nil, errors.Wrap(err, "Failed to create opensearch client")
	}
	return openSearchClient, err
}

func (o *OpenSearchLogProxy) ProxyFunctionLogs(ctx context.Context, options *platform.ProxyFunctionLogsOptions) (io.ReadCloser, error) {
	return nil, nil
}

func (o *OpenSearchLogProxy) GetFunctionReplicas(ctx context.Context, options *logProxy.GetFunctionReplicaOptions) ([]string, error) {
	return nil, nil
}
