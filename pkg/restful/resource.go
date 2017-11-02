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

package restful

import (
	"net/http"

	"github.com/nuclio/nuclio/pkg/registry"

	"github.com/go-chi/chi"
	"github.com/nuclio/nuclio-sdk"
)

type Attributes map[string]interface{}

// A custom route returns:
// resource type: string
// resources: a map of resource ID, resource attributes
// single: whether or not the resources should be treated as a single resource (if false, will be returned as list)
// status code: status code to return
// error: an error, if something went wrong
type CustomRouteFunc func(*http.Request) (string, map[string]Attributes, bool, int, error)

type CustomRoute struct {
	Method    string
	RouteFunc CustomRouteFunc
}

type Resource interface {

	// Called after initialization
	OnAfterInitialize()

	// returns a list of custom routes for the resource
	GetCustomRoutes() map[string]CustomRoute

	// return all instances for resources with multiple instances
	GetAll(request *http.Request) map[string]Attributes

	// return all instances for resources with single instances
	GetSingle(request *http.Request) (string, Attributes)

	// return specific instance by ID
	GetByID(request *http.Request, id string) Attributes

	// returns resource ID, attributes
	Create(request *http.Request) (string, Attributes, error)

	// returns attributes (optionally)
	Update(request *http.Request, id string) (Attributes, error)

	// delete an entity
	Remove(request *http.Request, id string) error
}

type ResourceMethod int

const (
	ResourceMethodGetList ResourceMethod = iota
	ResourceMethodGetDetail
	ResourceMethodCreate
	ResourceMethodUpdate
	ResourceMethodDelete
)

type AbstractResource struct {
	name            string
	Logger          nuclio.Logger
	router          chi.Router
	Resource        Resource
	resourceMethods []ResourceMethod
	server          interface{}
	encoderFactory  EncoderFactory
}

func NewAbstractResource(name string, resourceMethods []ResourceMethod) *AbstractResource {
	return &AbstractResource{
		name:            name,
		resourceMethods: resourceMethods,
		encoderFactory:  &JSONEncoderFactory{},
	}
}

func (ar *AbstractResource) Initialize(parentLogger nuclio.Logger, server interface{}) (chi.Router, error) {
	ar.Logger = parentLogger.GetChild(ar.name)

	ar.server = server
	ar.router = chi.NewRouter()

	// register routes based on supported methods
	ar.registerRoutes()

	ar.Resource.OnAfterInitialize()

	return ar.router, nil
}

func (ar *AbstractResource) Register(registry *registry.Registry) {
	registry.Register(ar.name, ar)
}

func (ar *AbstractResource) registerRoutes() error {
	for _, resourceMethod := range ar.resourceMethods {
		switch resourceMethod {
		case ResourceMethodGetList:
			ar.router.Get("/", ar.handleGetList)
		case ResourceMethodGetDetail:
			ar.router.Get("/{id}", ar.handleGetDetails)
		case ResourceMethodCreate:
			ar.router.Post("/", ar.handleCreate)
		case ResourceMethodUpdate:
			ar.router.Put("/{id}", ar.handleUpdate)
		case ResourceMethodDelete:
			ar.router.Delete("/{id}", ar.handleDelete)
		}
	}

	return ar.registerCustomRoutes()
}

func (ar *AbstractResource) GetServer() interface{} {
	return ar.server
}

func (ar *AbstractResource) registerCustomRoutes() error {
	CustomRouters := ar.Resource.GetCustomRoutes()

	// not all resources support custom routes
	if CustomRouters == nil {
		return nil
	}

	// iterate through the custom routes and register a handler for them
	for routePattern, CustomRoute := range CustomRouters {
		var routerFunc func(string, http.HandlerFunc)

		switch CustomRoute.Method {
		case http.MethodGet:
			routerFunc = ar.router.Get
		case http.MethodPost:
			routerFunc = ar.router.Post
		case http.MethodPut:
			routerFunc = ar.router.Put
		case http.MethodDelete:
			routerFunc = ar.router.Delete
		}

		CustomRouteCopy := CustomRoute

		routerFunc(routePattern, func(responseWriter http.ResponseWriter, request *http.Request) {
			ar.callCustomRouteFunc(responseWriter, request, CustomRouteCopy.RouteFunc)
		})
	}

	return nil
}

// called after initialization
func (ar *AbstractResource) OnAfterInitialize() {
}

// return all instances for resources with multiple instances
func (ar *AbstractResource) GetAll(request *http.Request) map[string]Attributes {
	return nil
}

// return all instances for resources with single instances
func (ar *AbstractResource) GetSingle(request *http.Request) (string, Attributes) {
	return "", nil
}

// return specific instance by ID
func (ar *AbstractResource) GetByID(request *http.Request, id string) Attributes {
	return nil
}

// create a resource
func (ar *AbstractResource) Create(request *http.Request) (string, Attributes, error) {
	return "", nil, nuclio.ErrNotImplemented
}

func (ar *AbstractResource) Update(request *http.Request, id string) (Attributes, error) {
	return nil, nuclio.ErrNotImplemented
}

func (ar *AbstractResource) Remove(request *http.Request, id string) error {
	return nuclio.ErrNotImplemented
}

// returns a list of custom routes for the resource
func (ar *AbstractResource) GetCustomRoutes() map[string]CustomRoute {
	return nil
}

// for raw routes, those that don't return an attribute
func (ar *AbstractResource) GetRouter() chi.Router {
	return ar.router
}

func (ar *AbstractResource) handleGetList(responseWriter http.ResponseWriter, request *http.Request) {
	encoder := ar.encoderFactory.NewEncoder(responseWriter, ar.name)

	// see if the resource only supports a single record
	singleResourceKey, singleResourceAttributes := ar.Resource.GetSingle(request)

	if singleResourceAttributes != nil {
		encoder.EncodeResource(singleResourceKey, singleResourceAttributes)

	} else {
		encoder.EncodeResources(ar.Resource.GetAll(request))
	}
}

func (ar *AbstractResource) handleGetDetails(responseWriter http.ResponseWriter, request *http.Request) {

	// registered as "/:id/"
	resourceID := chi.URLParam(request, "id")

	// delegate to child
	attributes := ar.Resource.GetByID(request, resourceID)

	// if not found return 404
	if attributes == nil {
		responseWriter.WriteHeader(http.StatusNotFound)
		return
	}

	ar.encoderFactory.NewEncoder(responseWriter, ar.name).EncodeResource(resourceID, attributes)
}

func (ar *AbstractResource) handleCreate(responseWriter http.ResponseWriter, request *http.Request) {

	// delegate to child
	resourceID, attributes, err := ar.Resource.Create(request)

	ar.setStatusCode(http.StatusCreated, err, responseWriter)

	// if no attributes given, return nothing
	if attributes == nil {
		return
	}

	ar.encoderFactory.NewEncoder(responseWriter, ar.name).EncodeResource(resourceID, attributes)
}

func (ar *AbstractResource) handleUpdate(responseWriter http.ResponseWriter, request *http.Request) {

	// registered as "/:id/"
	resourceID := chi.URLParam(request, "id")

	// delegate to child
	attributes, err := ar.Resource.Update(request, resourceID)

	// if no attributes given, return nothing
	if attributes == nil {
		ar.setStatusCode(http.StatusNoContent, err, responseWriter)
		return
	}

	ar.setStatusCode(http.StatusOK, err, responseWriter)

	ar.encoderFactory.NewEncoder(responseWriter, ar.name).EncodeResource(resourceID, attributes)
}

func (ar *AbstractResource) handleDelete(responseWriter http.ResponseWriter, request *http.Request) {

	// registered as "/:id/"
	resourceID := chi.URLParam(request, "id")

	// delegate to child
	err := ar.Resource.Remove(request, resourceID)

	// if not found return 404
	ar.setStatusCode(http.StatusNoContent, err, responseWriter)
}

func (ar *AbstractResource) callCustomRouteFunc(responseWriter http.ResponseWriter,
	request *http.Request,
	routeFunc CustomRouteFunc) {

	// see if the resource only supports a single record
	resourceType, resources, single, statusCode, _ := routeFunc(request)

	// set the status code
	responseWriter.WriteHeader(statusCode)

	if resources == nil {

		// write a valid, empty JSON
		responseWriter.Write([]byte("{}"))

		return
	}

	encoder := ar.encoderFactory.NewEncoder(responseWriter, resourceType)

	if single {

		// to get the first, we must iterate over range
		for resourceKey, resourceAttributes := range resources {
			encoder.EncodeResource(resourceKey, resourceAttributes)

			break
		}

	} else {
		encoder.EncodeResources(resources)
	}
}

func (ar *AbstractResource) setStatusCode(statusCode int, err error, responseWriter http.ResponseWriter) {
	if err != nil {
		errorWithStatusCode, errorHasStatusCode := err.(nuclio.ErrorWithStatusCode)
		if errorHasStatusCode {
			responseWriter.WriteHeader(errorWithStatusCode.StatusCode())
		}
	} else {
		responseWriter.WriteHeader(statusCode)
	}
}
