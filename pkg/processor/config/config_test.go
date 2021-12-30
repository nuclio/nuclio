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

package processorconfig

import (
	"bytes"
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor"

	"github.com/stretchr/testify/suite"
)

type WriterTestSuite struct {
	suite.Suite
	writer *Writer
	reader *Reader
}

func (suite *WriterTestSuite) SetupTest() {
	suite.writer, _ = NewWriter()
	suite.reader, _ = NewReader()
}

func (suite *WriterTestSuite) TestWrite() {
	output := bytes.Buffer{}

	writeConfiguration := processor.Configuration{
		Config: functionconfig.Config{
			Meta: functionconfig.Meta{
				Name: "name",
			},
			Spec: functionconfig.Spec{
				Description: "description",
				Handler:     "handler",
				Runtime:     "something:version",
			},
		},
	}

	suite.writer.Write(&output, &writeConfiguration) // nolint: errcheck

	readConfiguration := processor.Configuration{}

	input := bytes.NewBuffer(output.Bytes())
	suite.reader.Read(input, &readConfiguration) // nolint: errcheck

	suite.Require().Equal(writeConfiguration, readConfiguration)
}

func TestWriterTestSuite(t *testing.T) {
	suite.Run(t, new(WriterTestSuite))
}
