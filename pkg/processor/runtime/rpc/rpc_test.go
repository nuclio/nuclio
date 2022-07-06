//go:build test_unit

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
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type RPCSuite struct {
	suite.Suite
}

func (suite *RPCSuite) TestLogBeforeEvent() {
	suite.T().Skip()

	var sink bytes.Buffer
	var errSink bytes.Buffer
	logger, err := nucliozap.NewNuclioZap("RPCTest", "json", nil, &sink, &errSink, nucliozap.DebugLevel)
	suite.Require().NoError(err, "Can't create logger")

	var conn net.Conn

	_, err = NewAbstractRuntime(logger, suite.runtimeConfiguration(logger), nil)
	suite.Require().NoError(err, "Can't create RPC runtime")

	message := "testing log before"
	suite.emitLog(message, conn)
	time.Sleep(time.Millisecond) // Give TCP time to move bits around
	logger.Flush()
	suite.True(strings.Contains(sink.String(), message), "Didn't get log")
}

func (suite *RPCSuite) emitLog(message string, conn io.Writer) {
	log := &rpcLogRecord{
		DateTime: time.Now().String(),
		Level:    "info",
		Message:  message,
		With:     nil,
	}

	var buf bytes.Buffer
	buf.Write([]byte{'l'})
	enc := json.NewEncoder(&buf)
	err := enc.Encode(log)
	suite.Require().NoError(err, "Can't encode log record")
	_, err = io.Copy(conn, &buf)
	suite.Require().NoError(err)
}

func (suite *RPCSuite) dummyProcess() *os.Process {
	var buf bytes.Buffer
	cmd := exec.Command("ls")
	cmd.Stdout = &buf
	suite.Require().NoError(cmd.Run(), "Can't run")
	return cmd.Process
}

func (suite *RPCSuite) runtimeConfiguration(loggerInstance logger.Logger) *runtime.Configuration {
	return &runtime.Configuration{
		FunctionLogger: loggerInstance,
		Configuration: &processor.Configuration{
			Config: functionconfig.Config{
				Meta: functionconfig.Meta{
					Namespace: "test",
				},
				Spec: functionconfig.Spec{},
			},
			PlatformConfig: &platformconfig.Config{
				Kind: "docker",
			},
		},
	}
}

func TestRPC(t *testing.T) {
	suite.Run(t, new(RPCSuite))
}
