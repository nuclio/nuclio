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
	"github.com/satori/go.uuid"
	"io/ioutil"
)

//
// Function
//

type functionAttributes struct {
	State        string `json:"state"`
	SourceURL    string `json:"source_url"`
	DataBindings string `json:"data_bindings"`
}

type function struct {
	logger     nuclio.Logger
	id         uuid.UUID
	attributes functionAttributes
	runner     *runner.FunctionRunner
}

func newFunction(parentLogger nuclio.Logger, attributes *functionAttributes) (*function, error) {
	newFunction := &function{
		logger:     parentLogger.GetChild("function").(nuclio.Logger),
		id:         uuid.NewV4(),
		attributes: *attributes,
	}

	var err error

	// initialize runner options
	options := runner.Options{
		Build: builder.Options{
			Path:     attributes.SourceURL,
			Registry: "localhost:5000",
		},
		DataBindings: attributes.DataBindings,
	}

	// create a runner for the function
	newFunction.runner, err = runner.NewFunctionRunner(newFunction.logger, &options)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create function runner")
	}

	// update state
	newFunction.attributes.State = "Initializing"

	return newFunction, nil
}

func (f *function) getAttributes() restful.Attributes {
	return common.StructureToMap(f.attributes)
}

//
// Resource
//

type functionResource struct {
	*resource
	functions map[uuid.UUID]*function
}

// called after initialization
func (fr *functionResource) OnAfterInitialize() {
	fr.functions = map[uuid.UUID]*function{}
}

func (fr *functionResource) GetAll(request *http.Request) map[string]restful.Attributes {
	response := map[string]restful.Attributes{}

	for functionID, function := range fr.functions {
		response[functionID.String()] = function.getAttributes()
	}

	return response
}

// return specific instance by ID
func (fr *functionResource) GetByID(request *http.Request, id string) restful.Attributes {
	functionUUID, err := uuid.FromString(id)
	if err != nil {
		return nil
	}

	function, found := fr.functions[functionUUID]
	if !found {
		return nil
	}

	return function.getAttributes()
}

// returns resource ID, attributes
func (fr *functionResource) Create(request *http.Request) (id string, attributes restful.Attributes, responseErr error) {

	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		responseErr = nuclio.ErrInternalServerError
		return
	}

	functionAttributes := functionAttributes{}
	err = json.Unmarshal(body, &functionAttributes)
	if err != nil {
		responseErr = nuclio.ErrBadRequest
		return
	}

	// create a function
	newFunction, err := newFunction(fr.Logger, &functionAttributes)
	if err != nil {
		responseErr = nuclio.ErrInternalServerError
		return
	}

	// add function
	fr.functions[newFunction.id] = newFunction

	return newFunction.id.String(), newFunction.getAttributes(), nil
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
