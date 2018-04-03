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
	"runtime/debug"
	"sync"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/playground"
	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/nuclio/zap"
	"k8s.io/api/core/v1"
)

type functionAttributes struct {
	functionconfig.Config
	Status functionconfig.Status `json:"status,omitempty"`
}

type function struct {
	functionResource *functionResource
	logger           logger.Logger
	bufferLogger     *nucliozap.BufferLogger
	muxLogger        *nucliozap.MuxLogger
	platform         platform.Platform
	attributes       functionAttributes
}

func newFunction(parentLogger logger.Logger,
	functionResource *functionResource,
	functionConfig *functionconfig.Config,
	platform platform.Platform) (*function, error) {
	var err error

	newFunction := &function{
		logger:           parentLogger.GetChild(functionConfig.Meta.Name),
		functionResource: functionResource,
		attributes:       functionAttributes{Config: *functionConfig},
		platform:         platform,
	}

	newFunction.logger.InfoWith("Creating function")

	// create a buffer and mux logger for this function
	newFunction.bufferLogger, err = nucliozap.NewBufferLogger(functionConfig.Meta.Name, "json", nucliozap.InfoLevel)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create buffer logger")
	}

	newFunction.muxLogger, err = nucliozap.NewMuxLogger(newFunction.logger, newFunction.bufferLogger.Logger)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create buffer logger")
	}

	// update state
	newFunction.attributes.Status.State = functionconfig.FunctionStateWaitingForResourceConfiguration

	return newFunction, nil
}

func (f *function) Deploy() error {
	f.attributes.Status.State = functionconfig.FunctionStateWaitingForResourceConfiguration

	// deploy the runction
	deployResult, err := f.validateAndDeploy()

	if err != nil {
		f.attributes.Status.State = functionconfig.FunctionStateError
		f.attributes.Status.Message = fmt.Sprintf("Failed (%s)", errors.Cause(err).Error())
		f.muxLogger.WarnWith("Failed to deploy function", "err", errors.Cause(err))
	} else {
		f.attributes.Status.HTTPPort = deployResult.Port
		f.attributes.Status.State = functionconfig.FunctionStateReady
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
		marshalledLogs := f.bufferLogger.Buffer.String()

		// if something went wrong and there are no logs, do nothing
		if len(marshalledLogs) != 0 {

			marshalledLogs = "[" + marshalledLogs[:len(marshalledLogs)-1] + "]"

			// try to unmarshal the json
			err := json.Unmarshal([]byte(marshalledLogs), &f.attributes.Status.Logs)

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

func (f *function) validateAndDeploy() (*platform.CreateFunctionResult, error) {

	// a bit of validation prior
	if f.attributes.Meta.Namespace == "" {
		return nil, errors.New("Function namespace must be defined")
	}

	// deploy the runction
	return f.platform.CreateFunction(f.createDeployOptions())
}

func (f *function) createDeployOptions() *platform.CreateFunctionOptions {
	server := f.functionResource.GetServer().(*playground.Server)

	// initialize runner options and set defaults
	createFunctionOptions := &platform.CreateFunctionOptions{
		Logger:         f.logger,
		FunctionConfig: *functionconfig.NewConfig(),
	}

	readinessTimeout := 30 * time.Second

	createFunctionOptions.FunctionConfig = f.attributes.Config
	createFunctionOptions.FunctionConfig.Spec.Replicas = 1
	createFunctionOptions.FunctionConfig.Spec.Build.NoBaseImagesPull = server.NoPullBaseImages
	createFunctionOptions.Logger = f.muxLogger
	createFunctionOptions.FunctionConfig.Spec.Build.Path = "http://127.0.0.1:8070" + f.attributes.Spec.Build.Path
	createFunctionOptions.ReadinessTimeout = &readinessTimeout

	// if user provided registry, use that. Otherwise use default
	createFunctionOptions.FunctionConfig.Spec.Build.Registry = server.GetRegistryURL()
	if f.attributes.Spec.Build.Registry != "" {
		createFunctionOptions.FunctionConfig.Spec.Build.Registry = f.attributes.Spec.Build.Registry
	}

	// if user provided run registry, use that. if there's a default - use that. otherwise, use build registry
	if f.attributes.Spec.RunRegistry != "" {
		createFunctionOptions.FunctionConfig.Spec.RunRegistry = f.attributes.Spec.RunRegistry
	} else if server.GetRunRegistryURL() != "" {
		createFunctionOptions.FunctionConfig.Spec.RunRegistry = server.GetRunRegistryURL()
	} else {
		createFunctionOptions.FunctionConfig.Spec.RunRegistry = createFunctionOptions.FunctionConfig.Spec.Build.Registry
	}

	return createFunctionOptions
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
	isDeploying   bool
}

// called after initialization
func (fr *functionResource) OnAfterInitialize() error {
	fr.functions = map[string]*function{}
	fr.functionsLock = &sync.Mutex{}
	fr.platform = fr.getPlatform()

	for _, builtinFunctionConfig := range []functionconfig.Config{
		{
			Meta: functionconfig.Meta{
				Name: "echo",
			},
			Spec: functionconfig.Spec{
				Runtime: "golang",
				Build: functionconfig.Build{
					Path: "/sources/echo.go",
				},
			},
		},
		{
			Meta: functionconfig.Meta{
				Name: "encrypt",
			},
			Spec: functionconfig.Spec{
				Runtime: "python:3.6",
				Env: []v1.EnvVar{
					{Name: "ENCRYPT_KEY", Value: "correct_horse_battery_staple"},
				},
				Build: functionconfig.Build{
					Path: "/sources/encrypt.py",
					Commands: []string{
						"apk --update --no-cache add gcc g++ make libffi-dev openssl-dev",
						"pip install simple-crypt",
					},
				},
			},
		},
		{
			Meta: functionconfig.Meta{
				Name: "rabbitmq",
			},
			Spec: functionconfig.Spec{
				Runtime: "golang",
				Build: functionconfig.Build{
					Path: "/sources/rabbitmq.go",
				},
				Triggers: map[string]functionconfig.Trigger{
					"test_rmq": {
						Kind: "rabbit-mq",
						URL:  "amqp://user:password@rabbitmq-host:5672",
						Attributes: map[string]interface{}{
							"exchangeName": "exchange-name",
							"queueName":    "queue-name",
						},
					},
				},
			},
		},
		{
			Meta: functionconfig.Meta{
				Name: "face",
			},
			Spec: functionconfig.Spec{
				Runtime: "python:3.6",
				Env: []v1.EnvVar{
					{Name: "FACE_API_KEY", Value: "<key here>"},
					{Name: "FACE_API_BASE_URL", Value: "https://<region>.api.cognitive.microsoft.com/face/v1.0/"},
				},
				Build: functionconfig.Build{
					Path: "/sources/face.py",
					Commands: []string{
						"pip install cognitive_face tabulate inflection",
					},
				},
			},
		},
		{
			Meta: functionconfig.Meta{
				Name: "regexscan",
			},
			Spec: functionconfig.Spec{
				Runtime: "golang",
				Build: functionconfig.Build{
					Path: "/sources/regexscan.go",
				},
			},
		},
		{
			Meta: functionconfig.Meta{
				Name: "sentiments",
			},
			Spec: functionconfig.Spec{
				Runtime: "python:3.6",
				Build: functionconfig.Build{
					Path: "/sources/sentiments.py",
					Commands: []string{
						"pip install requests vaderSentiment",
					},
				},
			},
		},
		{
			Meta: functionconfig.Meta{
				Name: "tensorflow",
			},
			Spec: functionconfig.Spec{
				Runtime: "python:3.6",
				Build: functionconfig.Build{
					Path:      "/sources/tensor.py",
					BaseImage: "jessie",
					Commands: []string{
						"apt-get update && apt-get install -y wget",
						"wget http://download.tensorflow.org/models/image/imagenet/inception-2015-12-05.tgz",
						"mkdir -p /tmp/tfmodel",
						"tar -xzvf inception-2015-12-05.tgz -C /tmp/tfmodel",
						"rm inception-2015-12-05.tgz",
						"pip install requests numpy tensorflow",
					},
				},
			},
		},
		{
			Meta: functionconfig.Meta{
				Name: "img-convert",
			},
			Spec: functionconfig.Spec{
				Runtime: "shell",
				Handler: "convert",
				RuntimeAttributes: map[string]interface{}{
					"arguments": "- -resize 50% fd:1",
				},
				Build: functionconfig.Build{
					Path: "/sources/convert.sh",
					Commands: []string{
						"apk --update --no-cache add imagemagick",
					},
				},
			},
		},
		{
			Meta: functionconfig.Meta{
				Name: "dates",
			},
			Spec: functionconfig.Spec{
				Runtime: "nodejs",
				Build: functionconfig.Build{
					Path: "/sources/dates.js",
					Commands: []string{
						"npm install --global moment",
					},
				},
			},
		},
		{
			Meta: functionconfig.Meta{
				Name: "serialize-object",
			},
			Spec: functionconfig.Spec{
				Runtime: "dotnetcore",
				Handler: "nuclio:SerializeObject",
				Build: functionconfig.Build{
					Path: "/sources/SerializeObject.cs",
				},
			},
		},
		{
			Meta: functionconfig.Meta{
				Name: "reverser",
			},
			Spec: functionconfig.Spec{
				Runtime: "java",
				Handler: "ReverseEventHandler",
				Build: functionconfig.Build{
					Path: "/sources/ReverseEventHandler.java",
				},
			},
		},
		{
			Meta: functionconfig.Meta{
				Name: "s3watch",
			},
			Spec: functionconfig.Spec{
				Runtime: "golang",
				Handler: "Handler",
				Build: functionconfig.Build{
					Path: "/sources/s3watch.go",
				},
			},
		},
	} {
		builtinFunction := &function{}
		builtinFunction.attributes.Meta = builtinFunctionConfig.Meta
		builtinFunction.attributes.Meta.Namespace = "nuclio"
		builtinFunction.attributes.Spec = builtinFunctionConfig.Spec

		fr.functions[builtinFunctionConfig.Meta.Name] = builtinFunction
	}

	return nil
}

func (fr *functionResource) GetAll(request *http.Request) (map[string]restful.Attributes, error) {
	fr.functionsLock.Lock()
	defer fr.functionsLock.Unlock()

	response := map[string]restful.Attributes{}

	for functionID, function := range fr.functions {
		response[functionID] = function.getAttributes()
	}

	return response, nil
}

// return specific instance by ID
func (fr *functionResource) GetByID(request *http.Request, id string) (restful.Attributes, error) {
	fr.functionsLock.Lock()
	defer fr.functionsLock.Unlock()

	function, found := fr.functions[id]
	if !found {
		return nil, nil
	}

	readLogsTimeout := time.Second

	// update the logs (give it a second to be valid)
	function.ReadDeployerLogs(&readLogsTimeout)

	return function.getAttributes(), nil
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

	functionConfig := functionconfig.Config{}
	err = json.Unmarshal(body, &functionConfig)
	if err != nil {
		fr.Logger.WarnWith("Failed to parse JSON body", "err", err)

		responseErr = nuclio.ErrBadRequest
		return
	}

	// create a function
	newFunction, err := newFunction(fr.Logger, fr, &functionConfig, fr.platform)
	if err != nil {
		fr.Logger.WarnWith("Failed to create function", "err", err)

		responseErr = nuclio.ErrInternalServerError
		return
	}

	if fr.isDeploying {
		fr.Logger.Warn("Failed to deploy function - another function currently deploying")

		responseErr = nuclio.ErrTooManyRequests
		return
	}
	fr.isDeploying = true

	// run the function in the background
	go func() {
		defer fr.recoverFromDeploy()

		newFunction.Deploy() // nolint: errcheck
	}()

	// lock map while we're adding
	fr.functionsLock.Lock()
	defer fr.functionsLock.Unlock()

	// add function
	fr.functions[newFunction.attributes.Meta.Name] = newFunction

	return newFunction.attributes.Meta.Name, newFunction.getAttributes(), nil
}

func (fr *functionResource) recoverFromDeploy() {
	if r := recover(); r != nil {
		fr.Logger.ErrorWith("Recovered from panic during deploy",
			"err", r,
			"stack", string(debug.Stack()))
	}

	fr.isDeploying = false
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
