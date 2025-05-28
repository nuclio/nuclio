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
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/kube/logProxy"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
	"github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
)

type OpenSearchLogProxy struct {
	*AbstractSearchEngineLogProxy
	client *opensearchapi.Client
}

func NewOpenSearchLogProxy(config *platformconfig.ElasticSearchConfig) (*OpenSearchLogProxy, error) {
	openSearchClient := &OpenSearchLogProxy{
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
	if openSearchClient.client, err = opensearchapi.NewClient(opensearchapi.Config{Client: opensearch.Config{
		Addresses: []string{config.URL},
		Password:  config.Password,
		Username:  config.Username,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}}); err != nil {
		return nil, errors.Wrap(err, "Failed to create opensearch client")
	}

	return openSearchClient, err
}

func (o *OpenSearchLogProxy) GetFunctionReplicas(ctx context.Context, options *logProxy.GetFunctionReplicaOptions) ([]string, error) {
	if o.client == nil {
		return nil, errors.New("OpenSearch client is not configured")
	}
	query := o.getFunctionBaseSearchRequest(options.FunctionName)
	query["aggs"] = map[string]interface{}{
		"distinct_pod_names": map[string]interface{}{
			"terms": map[string]interface{}{
				"field": "kubernetes.pod.name",
				"size":  100000,
			},
		},
	}

	// Add time filter if provided
	o.addTimeFilter(query, options.TimeFilter)

	// Encode query to JSON
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(query); err != nil {
		return nil, errors.Wrap(err, "Failed to encode query")
	}

	searchRequest := &opensearchapi.SearchReq{
		Body: &buf,
	}

	// Perform search
	res, err := o.client.Search(ctx, searchRequest)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to execute search")
	}

	// Extract aggregation buckets
	var aggregatedPods struct {
		DistinctPodNames struct {
			Buckets []struct {
				Key string `json:"key"`
			} `json:"buckets"`
		} `json:"distinct_pod_names"`
	}

	if err := json.Unmarshal(res.Aggregations, &aggregatedPods); err != nil {
		return nil, errors.Wrap(err, "Failed to parse aggregation response")
	}

	replicas := make([]string, len(aggregatedPods.DistinctPodNames.Buckets))
	for i, bucket := range aggregatedPods.DistinctPodNames.Buckets {
		replicas[i] = bucket.Key
	}
	return replicas, nil
}

func (o *OpenSearchLogProxy) ProxyFunctionLogs(ctx context.Context, options *platform.ProxyFunctionLogsOptions) (io.ReadCloser, error) {
	if o.client == nil {
		return nil, errors.New("OpenSearch client is not configured")
	}

	query := o.getFunctionBaseSearchRequest(options.GetFunctionName())

	o.addTimeFilter(query, options.TimeFilter)

	if options.Substring != "" {
		o.addMustClause(query, "match_phrase", "message", options.Substring)
	}

	if options.Regexp != "" {
		o.addMustClause(query, "regexp", "message", options.Regexp)
	}

	o.addTermsFilter(query, "kubernetes.pod.name", options.ReplicaNames)
	o.addTermsFilter(query, "level", options.LogLevels)

	if options.Size != 0 {
		query["size"] = options.Size
	}

	if options.From != 0 {
		query["from"] = options.From
	}

	// Add search_after if specified
	if len(options.SearchAfter) > 0 {
		query["search_after"] = options.SearchAfter
	}

	searchRequest, err := o.getSearchRequestFromQuery(query)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create search request")
	}

	request, err := searchRequest.GetRequest()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to generate http request from search request")
	}

	// Perform search
	res, err := o.client.Client.Perform(request)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to execute search")
	}

	return res.Body, nil
}

func (o *OpenSearchLogProxy) getFunctionBaseSearchRequest(functionName string) map[string]interface{} {
	return map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []interface{}{
					map[string]interface{}{
						"wildcard": map[string]interface{}{
							"kubernetes.pod.name": fmt.Sprintf("nuclio-%s-*", functionName),
						},
					},
					map[string]interface{}{
						"query_string": map[string]interface{}{
							"query": o.customQueryParam,
						},
					},
				},
			},
		},
	}
}

func (o *OpenSearchLogProxy) getSearchRequestFromQuery(query map[string]interface{}) (*opensearchapi.SearchReq, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(query); err != nil {
		return nil, errors.Wrap(err, "Failed to encode query")
	}

	return &opensearchapi.SearchReq{
		Body: &buf,
	}, nil
}

func (o *OpenSearchLogProxy) addMustClause(query map[string]interface{}, clauseType, field, value string) {
	boolQuery := query["query"].(map[string]interface{})["bool"].(map[string]interface{})

	mustClause, ok := boolQuery["must"].([]interface{})
	if !ok {
		mustClause = []interface{}{}
	}

	boolQuery["must"] = append(mustClause, map[string]interface{}{
		clauseType: map[string]interface{}{
			field: value,
		},
	})
}

func (o *OpenSearchLogProxy) addTermsFilter(query map[string]interface{}, field string, values []string) {
	if len(values) == 0 {
		return
	}

	boolQuery := query["query"].(map[string]interface{})["bool"].(map[string]interface{})

	// Ensure "filter" exists and is a slice
	filters, ok := boolQuery["filter"].([]interface{})
	if !ok {
		filters = []interface{}{}
	}

	// Append the new terms filter
	filters = append(filters, map[string]interface{}{
		"terms": map[string]interface{}{
			field: values,
		},
	})

	// Reassign the updated filters slice
	boolQuery["filter"] = filters
}

func (o *OpenSearchLogProxy) addTimeFilter(query map[string]interface{}, timeFilter *platform.TimeFilter) {
	if timeFilter == nil {
		return
	}
	o.addTimeSort(query, timeFilter.Sort)

	timeRange := map[string]interface{}{}
	if timeFilter.Since != nil {
		timeRange["gte"] = timeFilter.Since.Format(time.RFC3339Nano)
	}
	if timeFilter.Until != nil {
		timeRange["lte"] = timeFilter.Until.Format(time.RFC3339Nano)
	}

	boolQuery := query["query"].(map[string]interface{})["bool"].(map[string]interface{})

	// Merge with existing filters
	filters, ok := boolQuery["filter"].([]interface{})
	if !ok {
		filters = []interface{}{}
	}

	filters = append(filters, map[string]interface{}{
		"range": map[string]interface{}{
			"@timestamp": timeRange,
		},
	})

	boolQuery["filter"] = filters
}
func (o *OpenSearchLogProxy) addTimeSort(query map[string]interface{}, sort string) {
	if sort == "" {
		return
	}

	newSortFields := []interface{}{
		map[string]interface{}{
			"@timestamp": map[string]interface{}{
				"order": sort,
			},
		},
		map[string]interface{}{
			"_id": map[string]interface{}{
				"order": sort,
			},
		},
	}

	// Try to append to existing sort if present
	if existingSort, ok := query["sort"].([]interface{}); ok {
		query["sort"] = append(existingSort, newSortFields...)
	} else {
		query["sort"] = newSortFields
	}
}
