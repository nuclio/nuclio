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

package elastic

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/kube/logProxy"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/sortorder"
	"github.com/nuclio/errors"
)

const distinctPodNamesFilter = "distinct_pod_names"

type ElasticSearchLogProxy struct {
	*AbstractSearchEngineLogProxy
	client *elasticsearch.TypedClient
}

func NewElasticSearchLogProxy(config *platformconfig.ElasticSearchConfig) (*ElasticSearchLogProxy, error) {
	esClient := &ElasticSearchLogProxy{
		AbstractSearchEngineLogProxy: &AbstractSearchEngineLogProxy{
			index:            config.Index,
			customQueryParam: config.CustomQueryParameter,
		},
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

func (e *ElasticSearchLogProxy) GetFunctionReplicas(ctx context.Context, options *logProxy.GetFunctionReplicaOptions) ([]string, error) {
	if e.client == nil {
		return nil, errors.New("Elasticsearch client is not configured")
	}

	searchRequest := e.getFunctionBaseSearchRequest(options.FunctionName)

	// Set to 0 to avoid fetching actual log documents (hits).
	// We're only interested in aggregation results (e.g., distinct pod names),
	// so setting this to 0 reduces response size, improves query performance,
	// and saves bandwidth and memory.
	searchRequest.Size = common.Pointer(0)

	if err := e.addTimeFilter(searchRequest, options.TimeFilter); err != nil {
		return nil, errors.Wrap(err, "Failed to apply time filter")
	}

	// aggregate by pod names
	searchRequest.Aggregations = map[string]types.Aggregations{
		distinctPodNamesFilter: {
			Terms: &types.TermsAggregation{
				Field: common.Pointer("kubernetes.pod.name"),
				Size:  common.Pointer(100000),
			},
		},
	}

	resp, err := e.client.Search().
		Index(e.index).
		Request(searchRequest).
		Do(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to execute search")
	}

	agg, ok := resp.Aggregations[distinctPodNamesFilter]
	if !ok {
		return nil, errors.New("Failed to get 'distinct_pod_names' in response from ElasticSearch")
	}

	// Convert to typed aggregation:
	termsAgg, ok := agg.(*types.StringTermsAggregate)
	if !ok {
		return nil, errors.New("Failed to convert aggregations to StringTermsAggregate")
	}

	replicas := make([]string, 0)

	switch buckets := termsAgg.Buckets.(type) {
	case []types.StringTermsBucket:
		for _, bucket := range buckets {
			replicas = append(replicas, bucket.Key.(string))
		}
	case map[string]types.StringTermsBucket:
		for _, bucket := range buckets {
			replicas = append(replicas, bucket.Key.(string))
		}
	default:
		return nil, errors.New("Failed to determine bucket format")
	}

	return replicas, nil

}

func (e *ElasticSearchLogProxy) ProxyFunctionLogs(ctx context.Context, options *platform.ProxyFunctionLogsOptions) (io.ReadCloser, error) {
	if e.client == nil {
		return nil, errors.New("Elasticsearch client is not configured")
	}
	searchRequest := e.getFunctionBaseSearchRequest(options.GetFunctionName())

	// Substring match
	if options.Substring != "" {
		searchRequest.Query.Bool.Must = append(searchRequest.Query.Bool.Must, types.Query{
			MatchPhrase: map[string]types.MatchPhraseQuery{
				"message": {Query: options.Substring},
			},
		})
	}

	// Regex match
	if options.Regexp != "" {
		searchRequest.Query.Bool.Must = append(searchRequest.Query.Bool.Must, types.Query{
			Regexp: map[string]types.RegexpQuery{
				"message": {Value: options.Regexp},
			},
		})
	}

	e.addTermsFilter(searchRequest, "kubernetes.pod.name", options.ReplicaNames)
	e.addTermsFilter(searchRequest, "level", options.LogLevels)

	if options.Size != 0 {
		searchRequest.Size = common.Pointer(int(options.Size))
	}

	if options.From != 0 {
		searchRequest.From = common.Pointer(int(options.From))
	}

	if err := e.addTimeFilter(searchRequest, options.TimeFilter); err != nil {
		return nil, errors.Wrap(err, "Failed to add time filter")
	}

	if len(options.SearchAfter) > 0 {
		for _, searchString := range options.SearchAfter {
			searchRequest.SearchAfter = append(searchRequest.SearchAfter, searchString)
		}
	}

	resp, err := e.client.Search().
		Index(e.index).
		Request(searchRequest).
		Perform(ctx)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to execute search request")
	}
	return resp.Body, nil
}

func (e *ElasticSearchLogProxy) getFunctionBaseSearchRequest(functionName string) *search.Request {
	searchRequest := search.NewRequest()

	// filters by function name custom query parameter
	query := types.Query{
		Bool: &types.BoolQuery{
			Must: []types.Query{
				{
					Wildcard: map[string]types.WildcardQuery{
						"kubernetes.pod.name": {
							Value: common.Pointer(fmt.Sprintf("nuclio-%s-*", functionName)),
						},
					},
				},
				{
					QueryString: &types.QueryStringQuery{
						Query: e.customQueryParam,
					},
				},
			},
		},
	}

	searchRequest.Query = &query
	return searchRequest
}

func (e *ElasticSearchLogProxy) addTermsFilter(searchRequest *search.Request, field string, values []string) {
	if len(values) == 0 {
		return
	}

	fieldValues := make([]types.FieldValue, len(values))
	for i, val := range values {
		fieldValues[i] = types.FieldValue(val)
	}

	query := types.Query{
		Terms: &types.TermsQuery{
			TermsQuery: map[string]types.TermsQueryField{
				field: types.TermsQueryField(fieldValues),
			},
		},
	}

	searchRequest.Query.Bool.Filter = append(searchRequest.Query.Bool.Filter, query)
}

func (e *ElasticSearchLogProxy) addTimeFilter(request *search.Request, timeFilter *platform.TimeFilter) error {
	if timeFilter == nil {
		return nil
	}
	if err := e.addTimeSort(request, timeFilter.Sort); err != nil {
		return errors.Wrap(err, "Failed to unmarshal sort order")
	}
	if timeFilter.Since != nil || timeFilter.Until != nil {
		rangeQuery := types.DateRangeQuery{}
		// Date fields are stored in Elasticsearch in a standard format based on ISO 8601
		// When you use range filters (like gte or lte), the values should be in this format:
		// "2023-11-21T14:00:00Z"
		if timeFilter.Since != nil {
			rangeQuery.Gte = common.Pointer(timeFilter.Since.Format(time.RFC3339Nano))
		}

		if timeFilter.Until != nil {
			rangeQuery.Lte = common.Pointer(timeFilter.Until.Format(time.RFC3339Nano))
		}

		request.Query.Bool.Filter = append(request.Query.Bool.Filter, types.Query{
			Range: map[string]types.RangeQuery{
				"@timestamp": rangeQuery,
			},
		})
	}
	return nil
}

func (e *ElasticSearchLogProxy) addTimeSort(request *search.Request, sort string) error {
	if sort != "" {
		var sortOrder sortorder.SortOrder
		if err := sortOrder.UnmarshalText([]byte(sort)); err != nil {
			return errors.Wrap(err, "Failed to unmarshal sort order")
		}
		request.Sort = []types.SortCombinations{
			&types.SortOptions{
				SortOptions: map[string]types.FieldSort{
					"@timestamp": {
						Order: &sortOrder,
					},
				},
			},
			&types.SortOptions{
				SortOptions: map[string]types.FieldSort{
					"_id": {
						Order: &sortOrder,
					},
				},
			},
		}
	}
	return nil
}
