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
	"sync"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/playground"
	"github.com/nuclio/nuclio/pkg/restful"
	"github.com/nuclio/nuclio/pkg/zap"

	"github.com/nuclio/nuclio-sdk"
	"k8s.io/api/core/v1"
)

//
// Function
//

type replicasAuto struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

type replicas struct {
	Static int          `json:"static"`
	Auto   replicasAuto `json:"auto"`
}

type resources struct {
	NumCPU int `json:"num_cpu"`
	Memory int `json:"memory"`
}

type logger struct {
	Level string `json:"level"`
}

type build struct {
	BaseImageName string   `json:"base_image_name"`
	Commands      []string `json:"commands"`
}

type functionAttributes struct {
	Name         string                                `json:"name"`
	Description  string                                `json:"description"`
	Enabled      bool                                  `json:"enabled"`
	Runtime      string                                `json:"runtime"`
	State        string                                `json:"state"`
	SourceURL    string                                `json:"source_url"`
	Registry     string                                `json:"registry"`
	RunRegistry  string                                `json:"run_registry"`
	Labels       map[string]string                     `json:"labels"`
	Env          map[string]string                     `json:"envs"`
	DataBindings map[string]functionconfig.DataBinding `json:"data_bindings"`
	Triggers     map[string]functionconfig.Trigger     `json:"triggers"`
	Replicas     replicas                              `json:"replicas"`
	NodePort     int                                   `json:"node_port"`
	Resources    resources                             `json:"resources"`
	Timeout      int                                   `json:"timeout"`
	Logger       logger                                `json:"level"`
	Build        build                                 `json:"build"`
	Logs         []map[string]interface{}              `json:"logs"`
}

type function struct {
	functionResource *functionResource
	logger           nuclio.Logger
	bufferLogger     *nucliozap.BufferLogger
	muxLogger        *nucliozap.MuxLogger
	attributes       functionAttributes
	platform         platform.Platform
}

func newFunction(parentLogger nuclio.Logger,
	functionResource *functionResource,
	attributes *functionAttributes,
	platform platform.Platform) (*function, error) {
	var err error

	newFunction := &function{
		logger:           parentLogger.GetChild(attributes.Name),
		functionResource: functionResource,
		attributes:       *attributes,
		platform:         platform,
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

func (f *function) Deploy() error {
	f.attributes.State = "Preparing"

	// create options
	deployResult, err := f.platform.DeployFunction(f.createDeployOptions())

	if err != nil {
		f.attributes.State = fmt.Sprintf("Failed (%s)", errors.Cause(err).Error())
		f.muxLogger.WarnWith("Failed to deploy function", "err", errors.Cause(err))
	} else {
		f.attributes.NodePort = deployResult.Port
		f.attributes.State = "Ready"
	}

	// read runner logs (no timeout - if we fail dont retry)
	f.ReadDeployerLogs(nil)

	return err
}

func (f *function) ReadDeployerLogs(timeout *time.Duration) {

	// if the function wasn't deployed yet, it won't have logs
	if f.bufferLogger == nil {
		return
	}

	deadline := time.Now()
	if timeout != nil {
		deadline = deadline.Add(*timeout)
	}

	// since the logs stream in, we can never know if they make for valid JSON. we can try until it works or unti
	// the deadline passes. if timeout is nil, we only try once
	for retryIndex := 0; true; retryIndex++ {

		// remove the last comma from the string
		marshalledLogs := string(f.bufferLogger.Buffer.Bytes())

		// if something went wrong and there are no logs, do nothing
		if len(marshalledLogs) != 0 {

			marshalledLogs = "[" + marshalledLogs[:len(marshalledLogs)-1] + "]"

			// try to unmarshal the json
			err := json.Unmarshal([]byte(marshalledLogs), &f.attributes.Logs)

			// if we got valid json we're done
			if err == nil {
				return
			}
		}

		// if we we're passed the deadline, we're done
		if time.Now().After(deadline) {
			return
		}

		// wait a bit and retry
		time.Sleep(time.Duration(25*retryIndex) * time.Millisecond)
	}
}

func (f *function) createDeployOptions() *platform.DeployOptions {
	server := f.functionResource.GetServer().(*playground.Server)

	// initialize runner options and set defaults
	deployOptions := &platform.DeployOptions{
		Logger:         f.logger,
		FunctionConfig: *functionconfig.NewConfig(),
	}

	deployOptions.FunctionConfig.Meta.Name = f.attributes.Name
	deployOptions.Logger = f.muxLogger
	deployOptions.FunctionConfig.Spec.Build.Path = "http://127.0.0.1:8070" + f.attributes.SourceURL
	deployOptions.FunctionConfig.Spec.Build.ImageName = f.attributes.Name
	deployOptions.FunctionConfig.Spec.DataBindings = f.attributes.DataBindings
	deployOptions.FunctionConfig.Spec.Triggers = f.attributes.Triggers
	deployOptions.FunctionConfig.Meta.Labels = f.attributes.Labels
	deployOptions.FunctionConfig.Spec.Replicas = 1
	deployOptions.FunctionConfig.Spec.Build.NoBaseImagesPull = server.NoPullBaseImages

	// if user provided registry, use that. Otherwise use default
	deployOptions.FunctionConfig.Spec.Build.Registry = server.GetRegistryURL()
	if f.attributes.Registry != "" {
		deployOptions.FunctionConfig.Spec.Build.Registry = f.attributes.Registry
	}

	// if user provided run registry, use that. if there's a default - use that. otherwise, use build registry
	if f.attributes.RunRegistry != "" {
		deployOptions.FunctionConfig.Spec.RunRegistry = f.attributes.RunRegistry
	} else if server.GetRunRegistryURL() != "" {
		deployOptions.FunctionConfig.Spec.RunRegistry = server.GetRunRegistryURL()
	} else {
		deployOptions.FunctionConfig.Spec.RunRegistry = deployOptions.FunctionConfig.Spec.Build.Registry
	}

	for envName, envValue := range f.attributes.Env {
		deployOptions.FunctionConfig.Spec.Env = append(deployOptions.FunctionConfig.Spec.Env, v1.EnvVar{
			Name:  envName,
			Value: envValue,
		})
	}

	return deployOptions
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
	platform      platform.Platform
}

// called after initialization
func (fr *functionResource) OnAfterInitialize() {
	fr.functions = map[string]*function{}
	fr.functionsLock = &sync.Mutex{}
	fr.platform = fr.getPlatform()

	// this is a bit of a hack, will be refactored later
	fr.functions["echo"] = &function{
		attributes: functionAttributes{
			Name:      "echo",
			SourceURL: "/sources/echo.go",
		},
	}

	fr.functions["encrypt"] = &function{
		attributes: functionAttributes{
			Name:      "encrypt",
			SourceURL: "/sources/encrypt.py",
			Env: map[string]string{
				"ENCRYPT_KEY": "correct_horse_battery_staple",
			},
		},
	}

	fr.functions["rabbitmq"] = &function{
		attributes: functionAttributes{
			Name:      "rabbitmq",
			SourceURL: "/sources/rabbitmq.go",
		},
	}

	fr.functions["face"] = &function{
		attributes: functionAttributes{
			Name:      "face",
			SourceURL: "/sources/face.py",
			Env: map[string]string{
				"FACE_API_KEY":      "<key here>",
				"FACE_API_BASE_URL": "https://<region>.api.cognitive.microsoft.com/face/v1.0/",
			},
		},
	}
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
	function.ReadDeployerLogs(&readLogsTimeout)

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
	newFunction, err := newFunction(fr.Logger, fr, &functionAttributesInstance, fr.platform)
	if err != nil {
		fr.Logger.WarnWith("Failed to create function", "err", err)

		responseErr = nuclio.ErrInternalServerError
		return
	}

	// run the function in the background
	go newFunction.Deploy()

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
