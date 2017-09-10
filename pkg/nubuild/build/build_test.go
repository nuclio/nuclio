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

package build

import (
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/zap"
)

var codeTemplate = `
package handler

import (
    "github.com/nuclio/nuclio-sdk"
)

func %s(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
}
`

type BuildSuite struct {
	suite.Suite
	logger nuclio.Logger
}

func (bs *BuildSuite) SetupSuite() {
	zap, err := nucliozap.NewNuclioZapTest("test-build")
	bs.Require().NoError(err, "Can't create logger")
	bs.logger = zap
}

func (bs *BuildSuite) TestHandlerName() {
	tmpDir, err := ioutil.TempDir("", "build-test")
	bs.Require().NoError(err, "Can't create temp dir")
	bs.logger.InfoWith("Temp directory", "path", tmpDir)
	goFile := fmt.Sprintf("%s/handler.go", tmpDir)
	handlerName := "HandleMessages" // Must start with capital letter
	code := fmt.Sprintf(codeTemplate, handlerName)
	err = ioutil.WriteFile(goFile, []byte(code), 0600)
	bs.Require().NoError(err, "Can't write code to %s", goFile)

	options := &Options{FunctionPath: tmpDir}
	builder := NewBuilder(bs.logger, options)

	cfg, err := builder.createConfig(goFile)
	bs.Require().NoError(err, "Can't read config")
	bs.Equal(cfg.Handler, handlerName, "Bad handler name")
}

func TestBuild(t *testing.T) {
	suite.Run(t, new(BuildSuite))
}
