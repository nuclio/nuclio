package main

import (
	"bytes"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	nuclio "github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/util/cmdrunner"
	nucliozap "github.com/nuclio/nuclio/pkg/zap"
)

type PythonHandlerSuite struct {
	suite.Suite

	logger         nuclio.Logger
	cmd            *cmdrunner.CmdRunner
	runtimeOptions *cmdrunner.RunOptions
}

func (suite *PythonHandlerSuite) failOnError(err error, fmt string, args ...interface{}) {
	if err == nil {
		return
	}
	suite.logger.ErrorWith("Error in test", "error", err)
	suite.FailNow(fmt, args...)
}

func (suite *PythonHandlerSuite) gitRoot() string {
	out, err := suite.cmd.Run(nil, "git rev-parse --show-toplevel")
	suite.failOnError(err, "Can't get git root")
	return strings.TrimSpace(out)
}

func (suite *PythonHandlerSuite) SetupSuite() {
	var loggerLevel nucliozap.Level

	if testing.Verbose() {
		loggerLevel = nucliozap.DebugLevel
	} else {
		loggerLevel = nucliozap.InfoLevel
	}
	zap, err := nucliozap.NewNuclioZap("end2end", loggerLevel)
	suite.failOnError(err, "Can't create logger")
	suite.logger = zap
	cmd, err := cmdrunner.NewCmdRunner(suite.logger)
	suite.failOnError(err, "Can't create command runner")
	suite.cmd = cmd
	gitRoot := suite.gitRoot()
	suite.runtimeOptions = &cmdrunner.RunOptions{WorkingDir: &gitRoot}

	suite.buildProcessor()
}

func (suite *PythonHandlerSuite) buildProcessor() {
	_, err := suite.cmd.Run(suite.runtimeOptions, "go build ./cmd/processor")
	suite.failOnError(err, "Can't build processor")
}

func (suite *PythonHandlerSuite) waitForHandler(url string, timeout time.Duration) {
	var err error

	start := time.Now()
	for time.Now().Sub(start) < timeout {
		_, err = http.Get(url)
		if err == nil {
			return
		}
		time.Sleep(time.Millisecond * 10)
	}
	suite.failOnError(err, "Can't call handler")
}

func (suite *PythonHandlerSuite) TestHandler() {
	cmd := exec.Command("./processor", "-config", "test/e2e/python/processor.yaml")
	cmd.Dir = *suite.runtimeOptions.WorkingDir
	cmd.Start()
	defer cmd.Process.Kill()

	handlerURL := "http://localhost:8080"
	suite.waitForHandler(handlerURL, 10*time.Second)

	rdr := strings.NewReader("ABCD")
	resp, err := http.Post(handlerURL, "text/plain", rdr)
	suite.failOnError(err, "Can't call controller")
	defer resp.Body.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, resp.Body)
	suite.failOnError(err, "Can't read body")
	suite.Equal("DCBA", buf.String())
}

func TestPythonHandler(t *testing.T) {
	suite.Run(t, new(PythonHandlerSuite))
}
