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

package resource

import (
	"net/http"

	"encoding/json"
	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/nuctl/builder"
	"github.com/nuclio/nuclio/pkg/nuctl/runner"
	"github.com/nuclio/nuclio/pkg/playground"
	"github.com/nuclio/nuclio/pkg/restful"
	"github.com/nuclio/nuclio/pkg/util/common"
	"github.com/pkg/errors"
	"io/ioutil"
	"github.com/nuclio/nuclio/pkg/nuctl"
	"bytes"
	"github.com/nuclio/nuclio/pkg/zap"
)

type bufferLogger struct {
	logger *nucliozap.NuclioZap
	writer *bytes.Buffer
}

//
// Function
//

type functionAttributes struct {
	Name         string `json:"name"`
	State        string `json:"state"`
	SourceURL    string `json:"source_url"`
	DataBindings string `json:"data_bindings"`
	Registry     string `json:"registry"`
	RunRegistry  string `json:"run_registry"`
	Log          []map[string]interface{} `json:"log"`
}

type function struct {
	logger     nuclio.Logger
	attributes functionAttributes
	runner     *runner.FunctionRunner
	bufferLogger *bufferLogger
}

func newFunction(parentLogger nuclio.Logger,
	bufferLogger *bufferLogger,
	attributes *functionAttributes) (*function, error) {

	var err error

	newFunction := &function{
		logger:     parentLogger.GetChild("function").(nuclio.Logger),
		attributes: *attributes,
		bufferLogger: bufferLogger,
	}

	newFunction.logger.InfoWith("Creating function")

	commonOptions := &nucliocli.CommonOptions{
		Identifier: "pgtest",
		KubeconfigPath: "/Users/erand/.kube/config",
	}

	// initialize runner options
	options := runner.Options{
		Common: commonOptions,
		Build: builder.Options{
			Common:   commonOptions,
			NuclioSourceURL: "https://github.com/nuclio/nuclio.git",
			Path:     attributes.SourceURL,
			Registry: attributes.Registry,
			OutputType: "docker",
			ImageVersion: "latest",
		},
		DataBindings: attributes.DataBindings,
		RunRegistry: attributes.RunRegistry,
	}

	// create a runner for the function
	newFunction.runner, err = runner.NewFunctionRunner(bufferLogger.logger, &options)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create function runner")
	}

	// update state
	newFunction.attributes.State = "Initializing"

	return newFunction, nil
}

func (f *function) Run() error {
	err := f.runner.Execute()
	if err != nil {
		f.bufferLogger.logger.WarnWith("Failed to run function", "err", errors.Cause(err))
	}

	// remove the last comma from the string
	marshalledLog := string(f.bufferLogger.writer.Bytes())
	marshalledLog = "[" + marshalledLog[:len(marshalledLog) - 1] + "]"

	// try to unmarshal the json
	return json.Unmarshal([]byte(marshalledLog), &f.attributes.Log)
}

func (f *function) getAttributes() restful.Attributes {
	return common.StructureToMap(f.attributes)
}

//
// Resource
//

type functionResource struct {
	*resource
	functions map[string]*function
	bufferLoggerChan chan *bufferLogger
}

// called after initialization
func (fr *functionResource) OnAfterInitialize() {
	fr.functions = map[string]*function{}

	fr.bufferLoggerChan, _ = fr.createBufferLoggerChan()
}

func (fr *functionResource) GetAll(request *http.Request) map[string]restful.Attributes {
	response := map[string]restful.Attributes{}

	for functionID, function := range fr.functions {
		response[functionID] = function.getAttributes()
	}

	return response
}

// return specific instance by ID
func (fr *functionResource) GetByID(request *http.Request, id string) restful.Attributes {
	function, found := fr.functions[id]
	if !found {
		return nil
	}

	return function.getAttributes()
}

// returns resource ID, attributes
func (fr *functionResource) Create(request *http.Request) (id string, attributes restful.Attributes, responseErr error) {

	// set the function logger to the runtime's logger capable of writing to a buffer
	// TODO: we should have a logger wrapper that can write to multiple loggers. this way function logs
	// get written both to the original function logger _and_ the HTTP stream
	bufferLogger := <-fr.bufferLoggerChan

	// set the logger level
	bufferLogger.logger.SetLevel(nucliozap.InfoLevel)

	// reset the buffer writer
	bufferLogger.writer.Reset()

	// read body
	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		fr.Logger.WarnWith("Failed to read body", "err", err)

		responseErr = nuclio.ErrInternalServerError
		return
	}

	functionAttributes := functionAttributes{}
	err = json.Unmarshal(body, &functionAttributes)
	if err != nil {
		fr.Logger.WarnWith("Failed to parse JSON body", "err", err)

		responseErr = nuclio.ErrBadRequest
		return
	}

	// create a function
	newFunction, err := newFunction(fr.Logger, bufferLogger, &functionAttributes)
	if err != nil {
		fr.Logger.WarnWith("Failed to create function", "err", err)

		responseErr = nuclio.ErrInternalServerError
		return
	}

	newFunction.Run()

	// add function. TODO: sync
	fr.functions[newFunction.attributes.Name] = newFunction

	return newFunction.attributes.Name, newFunction.getAttributes(), nil
}

func (fr *functionResource) createBufferLoggerChan() (chan *bufferLogger, error) {

	// will limit max number of concurrent HTTP invocations specifying logs returned
	// TODO: possibly from configuration
	numBufferLoggers := 4

	// create a channel for the buffer loggers
	bufferLoggersChan := make(chan *bufferLogger, numBufferLoggers)

	for bufferLoggerIdx := 0; bufferLoggerIdx < numBufferLoggers; bufferLoggerIdx++ {
		writer := &bytes.Buffer{}

		// create a logger that is able to capture the output into a buffer. if a request arrives
		// and the user wishes to capture the log, this will be used as the logger instead of the default
		// logger
		logger, err := nucliozap.NewNuclioZap("function",
			"json",
			writer,
			writer,
			nucliozap.DebugLevel)

		if err != nil {
			return nil, errors.Wrap(err, "Failed to create buffer logger")
		}

		// shove to channel
		bufferLoggersChan <- &bufferLogger{logger, writer}
	}

	return bufferLoggersChan, nil
}

// register the resource
var functionResourceInstance = &functionResource{
	resource: newResource("functions", []restful.ResourceMethod{
		restful.ResourceMethodGetList,
		restful.ResourceMethodGetDetail,
		restful.ResourceMethodCreate,
	}),
}

func init() {
	functionResourceInstance.Resource = functionResourceInstance
	functionResourceInstance.Register(playground.PlaygroundResourceRegistrySingleton)
}
