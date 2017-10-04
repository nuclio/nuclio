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
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/nuctl"
	"github.com/nuclio/nuclio/pkg/nuctl/executor"
	"github.com/nuclio/nuclio/pkg/nuctl/runner"
	"github.com/nuclio/nuclio/pkg/playground"
	"github.com/nuclio/nuclio/pkg/restful"
	"github.com/nuclio/nuclio/pkg/util/common"
	"github.com/nuclio/nuclio/pkg/zap"

	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/functioncr"
)

//
// Function
//

type functionAttributes struct {
	Name         string                            `json:"name"`
	State        string                            `json:"state"`
	SourceURL    string                            `json:"source_url"`
	DataBindings map[string]functioncr.DataBinding `json:"data_bindings"`
	Registry     string                            `json:"registry"`
	RunRegistry  string                            `json:"run_registry"`
	Logs         []map[string]interface{}          `json:"logs"`
	NodePort     int                               `json:"node_port"`
	Labels       map[string]string                 `json:"labels"`
	Env          map[string]string                 `json:"envs"`
}

type function struct {
	logger       nuclio.Logger
	bufferLogger *nucliozap.BufferLogger
	muxLogger    *nucliozap.MuxLogger
	attributes   functionAttributes
	kubeConsumer *nuctl.KubeConsumer
}

func newFunction(parentLogger nuclio.Logger,
	attributes *functionAttributes,
	kubeConsumer *nuctl.KubeConsumer) (*function, error) {
	var err error

	newFunction := &function{
		logger:       parentLogger.GetChild(attributes.Name).(nuclio.Logger),
		attributes:   *attributes,
		kubeConsumer: kubeConsumer,
	}

	newFunction.logger.InfoWith("Creating function")

	// create a buffer and mux logger for this function
	newFunction.bufferLogger, err = nucliozap.NewBufferLogger(attributes.Name, "json", nucliozap.InfoLevel)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create buffer logger")
	}

	newFunction.muxLogger, err = nucliozap.NewMuxLogger(newFunction.logger, newFunction.bufferLogger.Logger)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create buffer logger")
	}

	// update state
	newFunction.attributes.State = "Initializing"

	return newFunction, nil
}

func (f *function) Run() error {
	f.attributes.State = "Preparing"

	// create a runner using the kubeconsumer we got from the resource
	functionRunner, err := runner.NewFunctionRunner(f.muxLogger)
	if err != nil {
		return errors.Wrap(err, "Failed to create function runner")
	}

	// create options
	runnerOptions := f.createRunOptions()

	// execute the run
	runResult, err := functionRunner.Run(f.kubeConsumer, runnerOptions)

	if err != nil {
		f.attributes.State = fmt.Sprintf("Failed (%s)", errors.Cause(err).Error())
		f.muxLogger.WarnWith("Failed to run function", "err", errors.Cause(err))
	} else {
		f.attributes.NodePort = runResult.NodePort
		f.attributes.State = "Ready"
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
		marshalledLogs := string(f.bufferLogger.Buffer.Bytes())
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

func (f *function) createRunOptions() *runner.Options {
	commonOptions := &nuctl.CommonOptions{
		Identifier: f.attributes.Name,
	}

	// initialize runner options and set defaults
	runnerOptions := &runner.Options{Common: commonOptions}
	runnerOptions.InitDefaults()

	runnerOptions.Build.Path = f.attributes.SourceURL
	runnerOptions.Build.Registry = f.attributes.Registry
	runnerOptions.Build.ImageName = f.attributes.Name
	runnerOptions.DataBindings = f.attributes.DataBindings
	runnerOptions.Labels = common.StringMapToString(f.attributes.Labels)
	runnerOptions.Env = common.StringMapToString(f.attributes.Env)

	if f.attributes.RunRegistry != "" {
		runnerOptions.RunRegistry = f.attributes.RunRegistry
	} else {
		runnerOptions.RunRegistry = f.attributes.Registry
	}

	return runnerOptions
}

func (f *function) getAttributes() restful.Attributes {
	return common.StructureToMap(f.attributes)
}

//
// Resource
//

type functionResource struct {
	*resource
	functions     map[string]*function
	functionsLock sync.Locker
	executor      *executor.FunctionExecutor
	kubeConsumer  *nuctl.KubeConsumer
}

// called after initialization
func (fr *functionResource) OnAfterInitialize() {
	fr.functions = map[string]*function{}
	fr.functionsLock = &sync.Mutex{}

	// create kubeconsumer
	fr.kubeConsumer, _ = nuctl.NewKubeConsumer(fr.Logger, os.Getenv("KUBECONFIG"))
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

	// read body
	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		fr.Logger.WarnWith("Failed to read body", "err", err)

		responseErr = nuclio.ErrInternalServerError
		return
	}

	functionAttributesInstance := functionAttributes{}
	err = json.Unmarshal(body, &functionAttributesInstance)
	if err != nil {
		fr.Logger.WarnWith("Failed to parse JSON body", "err", err)

		responseErr = nuclio.ErrBadRequest
		return
	}

	// create a function
	newFunction, err := newFunction(fr.Logger, &functionAttributesInstance, fr.kubeConsumer)
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
