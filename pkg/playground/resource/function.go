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
	"encoding/json"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/nuctl"
	"github.com/nuclio/nuclio/pkg/nuctl/builder"
	"github.com/nuclio/nuclio/pkg/nuctl/runner"
	"github.com/nuclio/nuclio/pkg/playground"
	"github.com/nuclio/nuclio/pkg/restful"
	"github.com/nuclio/nuclio/pkg/util/common"
	"github.com/nuclio/nuclio/pkg/zap"

	"github.com/pkg/errors"
)

//
// Function
//

type functionAttributes struct {
	Name         string                   `json:"name"`
	State        string                   `json:"state"`
	SourceURL    string                   `json:"source_url"`
	DataBindings string                   `json:"data_bindings"`
	Registry     string                   `json:"registry"`
	RunRegistry  string                   `json:"run_registry"`
	Logs         []map[string]interface{} `json:"logs"`
}

type function struct {
	logger       nuclio.Logger
	muxLogger    nuclio.Logger
	bufferLogger *nucliozap.BufferLogger
	attributes   functionAttributes
	runner       *runner.FunctionRunner
}

func newFunction(parentLogger nuclio.Logger,
	bufferLogger *nucliozap.BufferLogger,
	attributes *functionAttributes) (*function, error) {

	var err error

	newFunction := &function{
		logger:       parentLogger.GetChild(attributes.Name).(nuclio.Logger),
		attributes:   *attributes,
		bufferLogger: bufferLogger,
	}

	// create a mux logger that will log both to buffer and the logger we received (stdout)
	newFunction.muxLogger, _ = nucliozap.NewMuxLogger(newFunction.logger, bufferLogger.Logger)

	newFunction.logger.InfoWith("Creating function")

	commonOptions := &nucliocli.CommonOptions{
		Identifier:     "pgtest",
		KubeconfigPath: "/Users/erand/.kube/config",
	}

	// initialize runner options
	options := runner.Options{
		Common: commonOptions,
		Build: builder.Options{
			Common:          commonOptions,
			NuclioSourceURL: "https://github.com/nuclio/nuclio.git",
			Path:            attributes.SourceURL,
			Registry:        attributes.Registry,
			OutputType:      "docker",
			ImageVersion:    "latest",
		},
		DataBindings: attributes.DataBindings,
		RunRegistry:  attributes.RunRegistry,
	}

	// create a runner for the function
	newFunction.runner, err = runner.NewFunctionRunner(newFunction.muxLogger, &options)
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
		f.muxLogger.WarnWith("Failed to run function", "err", errors.Cause(err))
	}

	// read runner logs (no timeout - if we fail dont retry)
	f.ReadRunnerLogs(nil)

	return err
}

func (f *function) ReadRunnerLogs(timeout *time.Duration) {
	deadline := time.Now()
	if timeout != nil {
		deadline = deadline.Add(*timeout)
	}

	// since the logs stream in, we can never know if they make for valid JSON. we can try until it works or unti
	// the deadline passes. if timeout is nil, we only try once
	for retryIndex := 0; true; retryIndex++ {

		// remove the last comma from the string
		marshalledLogs := f.bufferLogger.Read()
		marshalledLogs = "[" + marshalledLogs[:len(marshalledLogs)-1] + "]"

		// try to unmarshal the json
		err := json.Unmarshal([]byte(marshalledLogs), &f.attributes.Logs)

		// if we got valid json or we're passed the deadline, we're done
		if err == nil || time.Now().After(deadline) {
			return
		}

		// wait a bit and retry
		time.Sleep(time.Duration(25*retryIndex) * time.Millisecond)
	}
}

func (f *function) getAttributes() restful.Attributes {
	return common.StructureToMap(f.attributes)
}

//
// Resource
//

type functionResource struct {
	*resource
	functions        map[string]*function
	functionsLock    sync.Locker
	bufferLoggerPool *nucliozap.BufferLoggerPool
}

// called after initialization
func (fr *functionResource) OnAfterInitialize() {
	fr.functions = map[string]*function{}
	fr.functionsLock = &sync.Mutex{}

	// initialize the logger pool
	fr.bufferLoggerPool, _ = nucliozap.NewBufferLoggerPool(8,
		"function",
		"json",
		nucliozap.InfoLevel)
}

func (fr *functionResource) GetAll(request *http.Request) map[string]restful.Attributes {
	fr.functionsLock.Lock()
	defer fr.functionsLock.Unlock()

	response := map[string]restful.Attributes{}

	for functionID, function := range fr.functions {
		response[functionID] = function.getAttributes()
	}

	return response
}

// return specific instance by ID
func (fr *functionResource) GetByID(request *http.Request, id string) restful.Attributes {
	fr.functionsLock.Lock()
	defer fr.functionsLock.Unlock()

	function, found := fr.functions[id]
	if !found {
		return nil
	}

	readLogsTimeout := time.Second

	// update the logs (give it a second to be valid)
	function.ReadRunnerLogs(&readLogsTimeout)

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

	// run the function in the background
	go newFunction.Run()

	// lock map while we're adding
	fr.functionsLock.Lock()
	defer fr.functionsLock.Unlock()

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
