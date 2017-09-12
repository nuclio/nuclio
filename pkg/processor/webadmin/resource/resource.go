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
	"net/http"

	"github.com/nuclio/nuclio/cmd/processor/app"
	"github.com/nuclio/nuclio/pkg/processor/webadmin"

	"github.com/go-chi/chi"
	"github.com/nuclio/nuclio-sdk"
)

type attributes map[string]interface{}

// A custom route returns:
// resource type: string
// resources: a map of resource ID, resource attributes
// single: whether or not the resources should be treated as a single resource (if false, will be returned as list)
// status code: status code to return
// error: an error, if something went wrong
type customRouteFunc func(*http.Request) (string, map[string]attributes, bool, int, error)

type customRoute struct {
	method    string
	routeFunc customRouteFunc
}

type jsonapiResponse struct {
	Data interface{} `json:"data"`
}

type jsonapiResource struct {
	ID         string     `json:"id"`
	Type       string     `json:"type"`
	Attributes attributes `json:"attributes"`
}

type resource interface {

	// returns a list of custom routes for the resource
	getCustomRoutes() map[string]customRoute

	// return all instances for resources with multiple instances
	getAll(request *http.Request) map[string]attributes

	// return all instances for resources with single instances
	getSingle(request *http.Request) (string, attributes)

	// return specific instance by ID
	getByID(request *http.Request, id string) attributes

	// returns resource ID, attributes
	create(request *http.Request) (string, attributes, error)

	// returns attributes (optionally)
	update(request *http.Request, id string) (attributes, error)

	// delete an entity
	remove(request *http.Request, id string) error
}

type resourceMethod int

const (
	resourceMethodGetList resourceMethod = iota
	resourceMethodGetDetail
	resourceMethodCreate
	resourceMethodUpdate
	resourceMethodDelete
)

type abstractResource struct {
	name            string
	logger          nuclio.Logger
	processor       *app.Processor
	router          chi.Router
	resource        resource
	resourceMethods []resourceMethod
}

func newAbstractInterface(name string, resourceMethods []resourceMethod) *abstractResource {
	return &abstractResource{
		name:            name,
		resourceMethods: resourceMethods,
	}
}

func (ar *abstractResource) Initialize(parentLogger nuclio.Logger, processor interface{}) (chi.Router, error) {
	ar.logger = parentLogger.GetChild(ar.name).(nuclio.Logger)

	ar.processor = processor.(*app.Processor)
	ar.router = chi.NewRouter()

	// register routes based on supported methods
	ar.registerRoutes()

	return ar.router, nil
}

func (ar *abstractResource) register() {
	webadmin.ResourceRegistrySingleton.Register(ar.name, ar)
}

func (ar *abstractResource) registerRoutes() error {
	for _, resourceMethod := range ar.resourceMethods {
		switch resourceMethod {
		case resourceMethodGetList:
			ar.router.Get("/", ar.handleGetList)
		case resourceMethodGetDetail:
			ar.router.Get("/{id}", ar.handleGetDetails)
		case resourceMethodCreate:
			ar.router.Post("/", ar.handleCreate)
		case resourceMethodUpdate:
			ar.router.Put("/{id}", ar.handleUpdate)
		case resourceMethodDelete:
			ar.router.Delete("/{id}", ar.handleDelete)
		}
	}

	return ar.registerCustomRoutes()
}

func (ar *abstractResource) registerCustomRoutes() error {
	customRouters := ar.resource.getCustomRoutes()

	// not all resources support custom routes
	if customRouters == nil {
		return nil
	}

	// iterate through the custom routes and register a handler for them
	for routePattern, customRoute := range customRouters {
		var routerFunc func(string, http.HandlerFunc)

		switch customRoute.method {
		case http.MethodGet:
			routerFunc = ar.router.Get
		case http.MethodPost:
			routerFunc = ar.router.Post
		case http.MethodPut:
			routerFunc = ar.router.Put
		case http.MethodDelete:
			routerFunc = ar.router.Delete
		}

		customRouteCopy := customRoute

		routerFunc(routePattern, func(responseWriter http.ResponseWriter, request *http.Request) {
			ar.callCustomRouteFunc(responseWriter, request, customRouteCopy.routeFunc)
		})
	}

	return nil
}

// return all instances for resources with multiple instances
func (ar *abstractResource) getAll(request *http.Request) map[string]attributes {
	return nil
}

// return all instances for resources with single instances
func (ar *abstractResource) getSingle(request *http.Request) (string, attributes) {
	return "", nil
}

// return specific instance by ID
func (ar *abstractResource) getByID(request *http.Request, id string) attributes {
	return nil
}

// create a resource
func (ar *abstractResource) create(request *http.Request) (string, attributes, error) {
	return "", nil, nuclio.ErrNotImplemented
}

func (ar *abstractResource) update(request *http.Request, id string) (attributes, error) {
	return nil, nuclio.ErrNotImplemented
}

func (ar *abstractResource) remove(request *http.Request, id string) error {
	return nuclio.ErrNotImplemented
}

// returns a list of custom routes for the resource
func (ar *abstractResource) getCustomRoutes() map[string]customRoute {
	return nil
}

func (ar *abstractResource) handleGetList(responseWriter http.ResponseWriter, request *http.Request) {
	responseEncoder := json.NewEncoder(responseWriter)

	// see if the resource only supports a single record
	singleResourceKey, singleResourceAttributes := ar.resource.getSingle(request)

	if singleResourceAttributes != nil {

		responseEncoder.Encode(&jsonapiResponse{Data: jsonapiResource{
			Type:       ar.name,
			ID:         singleResourceKey,
			Attributes: singleResourceAttributes,
		}})

	} else {

		// resource supports multiple instances
		jsonapiResources := []jsonapiResource{}

		// delegate to child resource to get all
		for resourceKey, resourceAttributes := range ar.resource.getAll(request) {
			jsonapiResources = append(jsonapiResources, jsonapiResource{
				Type:       ar.name,
				ID:         resourceKey,
				Attributes: resourceAttributes,
			})
		}

		responseEncoder.Encode(&jsonapiResponse{Data: jsonapiResources})
	}
}

func (ar *abstractResource) handleGetDetails(responseWriter http.ResponseWriter, request *http.Request) {
	responseEncoder := json.NewEncoder(responseWriter)

	// registered as "/:id/"
	resourceID := chi.URLParam(request, "id")

	// delegate to child
	attributes := ar.resource.getByID(request, resourceID)

	// if not found return 404
	if attributes == nil {
		responseWriter.WriteHeader(http.StatusNotFound)
		return
	}

	responseEncoder.Encode(&jsonapiResponse{Data: jsonapiResource{
		Type:       ar.name,
		ID:         resourceID,
		Attributes: attributes,
	}})
}

func (ar *abstractResource) handleCreate(responseWriter http.ResponseWriter, request *http.Request) {
	responseEncoder := json.NewEncoder(responseWriter)

	// delegate to child
	resourceID, attributes, err := ar.resource.create(request)

	ar.setStatusCode(http.StatusCreated, err, responseWriter)

	// if no attributes given, return nothing
	if attributes == nil {
		return
	}

	responseEncoder.Encode(&jsonapiResponse{Data: jsonapiResource{
		Type:       ar.name,
		ID:         resourceID,
		Attributes: attributes,
	}})
}

func (ar *abstractResource) handleUpdate(responseWriter http.ResponseWriter, request *http.Request) {
	responseEncoder := json.NewEncoder(responseWriter)

	// registered as "/:id/"
	resourceID := chi.URLParam(request, "id")

	// delegate to child
	attributes, err := ar.resource.update(request, resourceID)

	// if no attributes given, return nothing
	if attributes == nil {
		ar.setStatusCode(http.StatusNoContent, err, responseWriter)
		return
	}

	ar.setStatusCode(http.StatusOK, err, responseWriter)

	responseEncoder.Encode(&jsonapiResponse{Data: jsonapiResource{
		Type:       ar.name,
		ID:         resourceID,
		Attributes: attributes,
	}})
}

func (ar *abstractResource) handleDelete(responseWriter http.ResponseWriter, request *http.Request) {

	// registered as "/:id/"
	resourceID := chi.URLParam(request, "id")

	// delegate to child
	err := ar.resource.remove(request, resourceID)

	// if not found return 404
	ar.setStatusCode(http.StatusNoContent, err, responseWriter)
}

func (ar *abstractResource) callCustomRouteFunc(responseWriter http.ResponseWriter,
	request *http.Request,
	routeFunc customRouteFunc) {

	responseEncoder := json.NewEncoder(responseWriter)

	// see if the resource only supports a single record
	resourceType, resources, single, statusCode, _ := routeFunc(request)

	// set the status code
	responseWriter.WriteHeader(statusCode)

	if resources == nil {

		// write a valid, empty JSON
		responseWriter.Write([]byte("{}"))

		return
	}

	// resource supports multiple instances
	jsonapiResources := []jsonapiResource{}

	// delegate to child resource to get all
	for resourceKey, resourceAttributes := range resources {
		jsonapiResources = append(jsonapiResources, jsonapiResource{
			Type:       resourceType,
			ID:         resourceKey,
			Attributes: resourceAttributes,
		})
	}

	if single {
		responseEncoder.Encode(&jsonapiResponse{Data: jsonapiResources[0]})
	} else {
		responseEncoder.Encode(&jsonapiResponse{Data: jsonapiResources})
	}
}

func (ar *abstractResource) setStatusCode(statusCode int, err error, responseWriter http.ResponseWriter) {
	if err != nil {
		errorWithStatusCode, errorHasStatusCode := err.(nuclio.ErrorWithStatusCode)
		if errorHasStatusCode {
			responseWriter.WriteHeader(errorWithStatusCode.StatusCode())
		}
	} else {
		responseWriter.WriteHeader(statusCode)
	}
}
