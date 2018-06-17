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

package rpc

import (
	"bytes"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type RuntimeSuite struct {
	suite.Suite
	wrapperProcess *os.Process
	wrapperConn    net.Conn
}

func (suite *RuntimeSuite) TestRestart() {
	logger := suite.createLogger()
	config := suite.createConfig()
	runtime, err := NewRPCRuntime(logger, config, suite.runWrapper, UnixSocket)
	suite.Require().NoError(err, "Can't create runtime")

	oldPid := suite.wrapperProcess.Pid
	err = runtime.Restart()
	suite.Require().NoError(err, "Can't restart runtime")
	suite.Require().NotEqual(oldPid, suite.wrapperProcess.Pid, "wrapper process didn't change")
}

func (suite *RuntimeSuite) TearDownTest() {
	if suite.wrapperProcess != nil {
		suite.wrapperProcess.Kill()
	}
}

func (suite *RuntimeSuite) createLogger() logger.Logger {
	var out bytes.Buffer

	logger, err := nucliozap.NewNuclioZap("rpc-runtime-test", "console", &out, &out, nucliozap.DebugLevel)
	suite.Require().NoError(err, "Can't create logger")

	return logger
}

func (suite *RuntimeSuite) createConfig() *runtime.Configuration {
	return &runtime.Configuration{
		Configuration: &processor.Configuration{
			Config: functionconfig.Config{
				Spec: functionconfig.Spec{},
			},
		},
	}
}

func (suite *RuntimeSuite) runWrapper(address string) (*os.Process, error) {
	var err error
	cmd := exec.Command("sleep", "999999")
	if err = cmd.Start(); err != nil {
		return nil, err
	}
	suite.wrapperProcess = cmd.Process

	// Connect to runtime
	suite.wrapperConn, err = net.Dial("unix", address)
	if err != nil {
		return nil, err
	}

	return cmd.Process, nil
}

func (suite *RuntimeSuite) sendLog(message string) {
	msg := map[string]interface{}{
		"level":    "INFO",
		"message":  message,
		"with":     nil,
		"datetime": time.Now().Format(time.RFC3339),
	}

	suite.wrapperConn.Write([]byte{'l'})
	json.NewEncoder(suite.wrapperConn).Encode(msg)
}

func TestRuntime(t *testing.T) {
	suite.Run(t, new(RuntimeSuite))
}
