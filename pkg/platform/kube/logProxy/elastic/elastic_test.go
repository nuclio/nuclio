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

/*
import (
	"context"
	"io"
	"testing"

	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/kube/logProxy"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type ElasticTestSuite struct {
	suite.Suite
	proxy  logProxy.LogProxy
	ctx    context.Context
	logger logger.Logger
}

// fill in the configuration for ElasticSearch
func (suite *ElasticTestSuite) SetupSuite() {
	var err error

	// create logger
	suite.logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)
	suite.proxy, err = ResolveLogProxy(suite.logger, &platformconfig.ElasticSearchConfig{
		URL:                  "",
		Username:             "",
		Password:             "",
		Index:                "filebeat-<system-id>-<namespace>-*",
		CustomQueryParameter: "system-id:\"system-id\"",
		SSLVerificationMode:  "none",
	})
	suite.Require().NoError(err, "Failed to create proxy")
	suite.ctx = context.Background()
}

func (suite *ElasticTestSuite) TestGetFunctionReplicas() {
	replicas, err := suite.proxy.GetFunctionReplicas(suite.ctx, &logProxy.GetFunctionReplicaOptions{FunctionName: "hello"})
	suite.Require().NoError(err)
	var logs io.ReadCloser
	options := platform.NewProxyFunctionLogsOptions("hello")
	options.Size = 100
	options.ReplicaNames = replicas
	options.LogLevels = []string{"debug"}
	logs, err = suite.proxy.ProxyFunctionLogs(suite.ctx, options)

	body, err := io.ReadAll(logs)
	suite.Require().NotEqual(0, len(body))

}

func TestElasticTestSuite(t *testing.T) {
	suite.Run(t, new(ElasticTestSuite))
}

*/
