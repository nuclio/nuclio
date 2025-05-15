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

/*
import (
	"context"
	"io"
	"testing"

	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/stretchr/testify/suite"
)

type ElasticTestSuite struct {
	suite.Suite
	proxier *ElasticLogProxy
	ctx     context.Context
}

func (suite *ElasticTestSuite) SetupSuite() {
	var err error
	suite.proxier, err = NewElasticLogProxy(&platformconfig.ElasticSearchConfig{
		URL:                  "",
		Username:             "",
		Password:             "",
		Index:                "filebeat-vmdev93-default-tenant-*",
		CustomQueryParameter: "system-id=\"vmdev93\"",
		SSLVerificationMode:  "none",
	})
	suite.Require().NoError(err, "Failed to create proxier")
	suite.ctx = context.Background()
}

func (suite *ElasticTestSuite) TestGetFunctionReplicas() {
	replicas, err := suite.proxier.GetFunctionReplicas(suite.ctx, &GetFunctionReplicaOptions{FunctionName: "hello"})
	suite.Require().NoError(err)
	//suite.Require().Len(replicas, 1)
	var logs io.ReadCloser
	//yday := time.Date(2025, 5, 13, 0, 0, 0, 0, time.UTC)
	options := platform.NewProxyFunctionLogsOptions("hello")
	options.Size = 100
	options.ReplicaNames = replicas
	options.LogLevels = []string{"debug"}
	logs, err = suite.proxier.ProxyFunctionLogs(suite.ctx, options)

	body, err := io.ReadAll(logs)
	suite.Require().NotEqual(0, len(body))

}

func TestElasticTestSuite(t *testing.T) {
	suite.Run(t, new(ElasticTestSuite))
}

*/
