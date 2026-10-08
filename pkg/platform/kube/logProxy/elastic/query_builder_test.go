//go:build test_unit

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
	"testing"

	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/stretchr/testify/suite"
)

const testProjectNameField = "kubernetes.labels.nuclio_io/project-name"

type QueryBuilderTestSuite struct {
	suite.Suite
}

func (suite *QueryBuilderTestSuite) TestElasticSearchProjectFilterAddedWhenConfigured() {
	proxy, err := NewElasticSearchLogProxy(&platformconfig.ElasticSearchConfig{
		Index:            "filebeat-*",
		ProjectNameField: testProjectNameField,
	})
	suite.Require().NoError(err)

	searchRequest := proxy.getFunctionBaseSearchRequest("myfunc", "proj-a")

	// pod-name wildcard scoping is unchanged
	suite.Require().Len(searchRequest.Query.Bool.Must, 2)
	suite.Equal("nuclio-myfunc-*", *searchRequest.Query.Bool.Must[0].Wildcard[kubernetesPodNameKey].Value)

	// project-name safety filter was added
	suite.Require().Len(searchRequest.Query.Bool.Filter, 1)
	termsQuery := searchRequest.Query.Bool.Filter[0].Terms
	suite.Require().NotNil(termsQuery)
	suite.Equal(
		types.TermsQueryField([]types.FieldValue{"proj-a"}),
		termsQuery.TermsQuery[testProjectNameField],
	)
}

func (suite *QueryBuilderTestSuite) TestElasticSearchProjectFilterSkippedWhenFieldNotConfigured() {
	proxy, err := NewElasticSearchLogProxy(&platformconfig.ElasticSearchConfig{
		Index: "filebeat-*",
	})
	suite.Require().NoError(err)

	searchRequest := proxy.getFunctionBaseSearchRequest("myfunc", "proj-a")

	suite.Empty(searchRequest.Query.Bool.Filter)
}

func (suite *QueryBuilderTestSuite) TestElasticSearchProjectFilterSkippedWhenProjectNameEmpty() {
	proxy, err := NewElasticSearchLogProxy(&platformconfig.ElasticSearchConfig{
		Index:            "filebeat-*",
		ProjectNameField: testProjectNameField,
	})
	suite.Require().NoError(err)

	searchRequest := proxy.getFunctionBaseSearchRequest("myfunc", "")

	suite.Empty(searchRequest.Query.Bool.Filter)
}

func (suite *QueryBuilderTestSuite) TestOpenSearchProjectFilterAddedWhenConfigured() {
	proxy, err := NewOpenSearchLogProxy(&platformconfig.ElasticSearchConfig{
		Index:            "filebeat-*",
		ProjectNameField: testProjectNameField,
	})
	suite.Require().NoError(err)

	query := proxy.getFunctionBaseSearchRequest("myfunc", "proj-a")
	boolQuery := query["query"].(map[string]interface{})["bool"].(map[string]interface{})

	// pod-name wildcard scoping is unchanged
	must := boolQuery["must"].([]interface{})
	suite.Require().Len(must, 2)
	wildcardClause := must[0].(map[string]interface{})["wildcard"].(map[string]interface{})
	suite.Equal("nuclio-myfunc-*", wildcardClause[kubernetesPodNameKey])

	// project-name safety filter was added
	filter := boolQuery["filter"].([]interface{})
	suite.Require().Len(filter, 1)
	termsClause := filter[0].(map[string]interface{})["terms"].(map[string]interface{})
	suite.Equal([]string{"proj-a"}, termsClause[testProjectNameField])
}

func (suite *QueryBuilderTestSuite) TestOpenSearchProjectFilterSkippedWhenFieldNotConfigured() {
	proxy, err := NewOpenSearchLogProxy(&platformconfig.ElasticSearchConfig{
		Index: "filebeat-*",
	})
	suite.Require().NoError(err)

	query := proxy.getFunctionBaseSearchRequest("myfunc", "proj-a")
	boolQuery := query["query"].(map[string]interface{})["bool"].(map[string]interface{})

	suite.NotContains(boolQuery, "filter")
}

func (suite *QueryBuilderTestSuite) TestOpenSearchProjectFilterSkippedWhenProjectNameEmpty() {
	proxy, err := NewOpenSearchLogProxy(&platformconfig.ElasticSearchConfig{
		Index:            "filebeat-*",
		ProjectNameField: testProjectNameField,
	})
	suite.Require().NoError(err)

	query := proxy.getFunctionBaseSearchRequest("myfunc", "")
	boolQuery := query["query"].(map[string]interface{})["bool"].(map[string]interface{})

	suite.NotContains(boolQuery, "filter")
}

func TestQueryBuilderTestSuite(t *testing.T) {
	suite.Run(t, new(QueryBuilderTestSuite))
}
