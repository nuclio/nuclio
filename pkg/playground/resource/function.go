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
	"io/ioutil"
	"encoding/json"

	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/nuctl/builder"
	"github.com/nuclio/nuclio/pkg/nuctl/runner"
	"github.com/nuclio/nuclio/pkg/playground"
	"github.com/nuclio/nuclio/pkg/restful"
	"github.com/nuclio/nuclio/pkg/util/common"
	"github.com/nuclio/nuclio/pkg/nuctl"
	"github.com/nuclio/nuclio/pkg/zap"

	"github.com/pkg/errors"
)

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
	Logs         []map[string]interface{} `json:"logs"`
}

type function struct {
	logger     nuclio.Logger
	attributes functionAttributes
	runner     *runner.FunctionRunner
	bufferLogger *nucliozap.BufferLogger
}

func newFunction(parentLogger nuclio.Logger,
	bufferLogger *nucliozap.BufferLogger,
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

	// create a mux logger that will log both to buffer and ourselves
	muxLogger, _ := nucliozap.NewMuxLogger(newFunction.logger, bufferLogger.Logger)

	// create a runner for the function
	newFunction.runner, err = runner.NewFunctionRunner(muxLogger, &options)
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
		f.bufferLogger.Logger.WarnWith("Failed to run function", "err", errors.Cause(err))
	}

	// remove the last comma from the string
	marshalledLogs := string(f.bufferLogger.Writer.Bytes())
	marshalledLogs = "[" + marshalledLogs[:len(marshalledLogs) - 1] + "]"

	// try to unmarshal the json
	return json.Unmarshal([]byte(marshalledLogs), &f.attributes.Logs)
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
	bufferLoggerPool *nucliozap.BufferLoggerPool
}

// called after initialization
func (fr *functionResource) OnAfterInitialize() {
	fr.functions = map[string]*function{}

	// initialize the logger pool
	fr.bufferLoggerPool, _ = nucliozap.NewBufferLoggerPool(8,
		"function",
		"json",
		nucliozap.InfoLevel)
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

	// allocate a buffer logger
	bufferLogger, err := fr.bufferLoggerPool.Allocate(nil)
	if err != nil {
		fr.Logger.WarnWith("Failed to allocate logger", "err", err)
		responseErr = nuclio.ErrInternalServerError
		return
	}

	defer fr.bufferLoggerPool.Release(bufferLogger)

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

	// run the function
	newFunction.Run()

	// add function
	fr.functions[newFunction.attributes.Name] = newFunction

	return newFunction.attributes.Name, newFunction.getAttributes(), nil
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
