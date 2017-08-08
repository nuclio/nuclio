package build

import (
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/nuclio/nuclio-sdk"
	nucliozap "github.com/nuclio/nuclio/pkg/zap"
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

func (bs *BuildSuite) failOnError(err error, fmt string, args ...interface{}) {
	if err != nil {
		bs.FailNowf(err.Error(), fmt, args...)
	}
}

func (bs *BuildSuite) SetupSuite() {
	var loggerLevel nucliozap.Level
	if testing.Verbose() {
		loggerLevel = nucliozap.DebugLevel
	} else {
		loggerLevel = nucliozap.InfoLevel
	}

	zap, err := nucliozap.NewNuclioZap("test-build", loggerLevel)
	bs.failOnError(err, "Can't create logger")
	bs.logger = zap
}

func (bs *BuildSuite) TestHandlerName() {
	tmpDir, err := ioutil.TempDir("", "build-test")
	bs.failOnError(err, "Can't create temp dir")
	bs.logger.InfoWith("Temp directory", "path", tmpDir)
	goFile := fmt.Sprintf("%s/handler.go", tmpDir)
	handlerName := "HandleMessages" // Must start with capital letter
	code := fmt.Sprintf(codeTemplate, handlerName)
	err = ioutil.WriteFile(goFile, []byte(code), 0600)
	bs.failOnError(err, "Can't write code to %s", goFile)

	options := &Options{FunctionPath: tmpDir}
	builder := NewBuilder(bs.logger, options)

	cfg, err := builder.readConfig("/no/such/file", "/no/such/file")
	bs.failOnError(err, "Can't read config")
	bs.Equal(cfg.Handler, handlerName, "Bad handler name")
}

func TestBuild(t *testing.T) {
	suite.Run(t, new(BuildSuite))
}
