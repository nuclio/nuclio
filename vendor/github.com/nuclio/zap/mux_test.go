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

package nucliozap

import (
	"testing"

	"github.com/nuclio/logger"
	"github.com/stretchr/testify/suite"
)

type MuxLoggerTestSuite struct {
	suite.Suite
	bufferLoggers []*BufferLogger
	loggers       []logger.Logger
}

func (suite *MuxLoggerTestSuite) SetupTest() {
	suite.bufferLoggers = suite.bufferLoggers[:0]
	suite.loggers = suite.loggers[:0]

	// create a few loggers so that we can use them
	for bufferIdx := 0; bufferIdx < 3; bufferIdx++ {
		bufferLogger, err := NewBufferLogger("bl", "json", InfoLevel)
		suite.Require().NoError(err)

		suite.bufferLoggers = append(suite.bufferLoggers, bufferLogger)
		suite.loggers = append(suite.loggers, bufferLogger.Logger)
	}
}

func (suite *MuxLoggerTestSuite) TestLoggingFromNew() {

	// create logger
	muxLogger, err := NewMuxLogger(suite.loggers...)
	suite.Require().NoError(err)

	suite.logAndVerify(muxLogger)
}

func (suite *MuxLoggerTestSuite) TestLoggingFromSet() {

	// create logger
	muxLogger, err := NewMuxLogger()
	suite.Require().NoError(err)

	muxLogger.SetLoggers(suite.loggers...)

	suite.logAndVerify(muxLogger)
}

func (suite *MuxLoggerTestSuite) logAndVerify(muxLogger *MuxLogger) {

	// log three messages (though level is info
	muxLogger.DebugWith("Debug", "id", 0)
	muxLogger.InfoWith("Info", "id", 1)
	muxLogger.WarnWith("Warn", "id", 2)

	// verify logs reached all buffers
	for _, bufferLogger := range suite.bufferLoggers {
		logEntries, err := bufferLogger.GetLogEntries()
		suite.Require().NoError(err)
		suite.Require().Len(logEntries, 2)

		suite.Require().Equal(logEntries[0]["message"], "Info")
		suite.Require().Equal(logEntries[1]["message"], "Warn")
	}
}

func TestMuxLoggerTestSuite(t *testing.T) {
	suite.Run(t, new(MuxLoggerTestSuite))
}
