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

package config

import (
	"testing"

	"github.com/nuclio/nuclio-sdk"
	"github.com/stretchr/testify/suite"
)

type WriterTestSuite struct {
	suite.Suite
	logger nuclio.Logger
	writer *Writer
}

func (suite *WriterTestSuite) SetupTest() {
	suite.writer = NewWriter()
}

func (suite *WriterTestSuite) TestWrite() {
	//output := bytes.Buffer{}
	//
	//suite.writer.Write(&output,
	//	"handler_",
	//	"runtime_",
	//	"logLevel_",
	//	map[string]platform.DataBinding{
	//		"db0_": {
	//			Class: "db0_class_",
	//			URL:   "db0_url_",
	//		},
	//		"db1_": {
	//			Class: "db1_class_",
	//			URL:   "db1_url_",
	//		},
	//	},
	//	map[string]platform.Trigger{
	//		"t0": {
	//			Class:    "t0_class_",
	//			Kind:     "t0_kind_",
	//			Disabled: true,
	//			Attributes: map[string]interface{}{
	//				"t0_attr1_key": "t0_attr1_value",
	//				"t0_attr2_key": 100,
	//			},
	//		},
	//		"t1": {
	//			Class:    "t1_class_",
	//			Kind:     "t1_kind_",
	//			Disabled: false,
	//		},
	//	})
	//
	//fmt.Println(output.String())
	// TODO
}

func TestWriterTestSuite(t *testing.T) {
	suite.Run(t, new(WriterTestSuite))
}
