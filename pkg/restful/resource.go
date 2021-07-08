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
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/registry"

	"github.com/go-chi/chi/v5"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

// Attributes are resource attributes
type Attributes map[string]interface{}

// CustomRouteFuncResponse is what CustomRouteFunc returns
type CustomRouteFuncResponse struct {
	ResourceType string
	Resources    map[string]Attributes
	Headers      map[string]string
	// Whether or not the resources should be treated as a single resource (if
	// false, will be returned as list)
	Single     bool
	StatusCode int
}

// CustomRouteFuncStreamResponse is what CustomRouteFuncStream returns
type CustomRouteFuncStreamResponse struct {
	Headers       map[string]string
	StatusCode    int
	ForceFlush    bool
	FlushInternal time.Duration
	ReadCloser    io.ReadCloser
}

// CustomRouteFunc is a handler function for a custom route
type CustomRouteFunc func(*http.Request) (*CustomRouteFuncResponse, error)

// CustomRouteFuncStream is a handler function for a custom route server stream response
type CustomRouteFuncStream func(*http.Request) (*CustomRouteFuncStreamResponse, error)

// CustomRoute is a custom route definition
type CustomRoute struct {
	Stream          bool
	Pattern         string
	Method          string
	RouteFunc       CustomRouteFunc
	StreamRouteFunc CustomRouteFuncStream
}

// Resource interface
type Resource interface {

	// Initialize the concrete server
	Initialize(logger.Logger, Server) (chi.Router, error)

	// Called after initialization
	OnAfterInitialize() error

	// returns a list of custom routes for the resource
	GetCustomRoutes() ([]CustomRoute, error)

	// return all instances for resources with multiple instances
	GetAll(request *http.Request) (map[string]Attributes, error)

	// return specific instance by ID
	GetByID(request *http.Request, id string) (Attributes, error)

	// returns resource ID, attributes
	Create(request *http.Request) (string, Attributes, error)

	// returns attributes (optionally)
	Update(request *http.Request, id string) (Attributes, error)

	// delete an entity
	Delete(request *http.Request, id string) error
}

// ResourceMethod is the method of the resource
type ResourceMethod int

// Possible resource methods
const (
	ResourceMethodGetList ResourceMethod = iota
	ResourceMethodGetDetail
	ResourceMethodCreate
	ResourceMethodUpdate
	ResourceMethodDelete
)

const (
	ParamImport = "import"
	ParamExport = "export"
)

// AbstractResource is base for resources
type AbstractResource struct {
	name            string
	Logger          logger.Logger
	router          chi.Router
	Resource        Resource
	resourceMethods []ResourceMethod
	server          Server
	encoderFactory  EncoderFactory
}

// NewAbstractResource creates a new AbstractResource
func NewAbstractResource(name string, resourceMethods []ResourceMethod) *AbstractResource {
	return &AbstractResource{
		name:            name,
		resourceMethods: resourceMethods,
		encoderFactory:  &JSONEncoderFactory{},
	}
}

// Initialize initializes the resource
func (ar *AbstractResource) Initialize(parentLogger logger.Logger, server Server) (chi.Router, error) {
	ar.Logger = parentLogger.GetChild(ar.name)

	ar.server = server
	ar.router = chi.NewRouter()

	// register routes based on supported methods
	if err := ar.registerRoutes(); err != nil {
		return nil, errors.Wrap(err, "Failed to register routes")
	}

	if err := ar.Resource.OnAfterInitialize(); err != nil {
		return nil, errors.Wrap(err, "OnAfterInitialize returned error")
	}

	return ar.router, nil
}

// Register registers a registry
func (ar *AbstractResource) Register(registry *registry.Registry) {
	registry.Register(ar.name, ar)
}

// GetServer returns the server
func (ar *AbstractResource) GetServer() Server {
	return ar.server
}

// OnAfterInitialize is called after initialization
func (ar *AbstractResource) OnAfterInitialize() error {
	return nil
}

// GetAll returns all instances for resources with multiple instances
func (ar *AbstractResource) GetAll(request *http.Request) (map[string]Attributes, error) {
	return nil, nil
}

// GetByID return specific instance by ID
func (ar *AbstractResource) GetByID(request *http.Request, id string) (Attributes, error) {
	return nil, nil
}

// Create a resource
func (ar *AbstractResource) Create(request *http.Request) (string, Attributes, error) {
	return "", nil, nuclio.ErrNotImplemented
}

// Update a resource
func (ar *AbstractResource) Update(request *http.Request, id string) (Attributes, error) {
	return nil, nuclio.ErrNotImplemented
}

// Delete a resource
func (ar *AbstractResource) Delete(request *http.Request, id string) error {
	return nuclio.ErrNotImplemented
}

// GetCustomRoutes returns a list of custom routes for the resource
func (ar *AbstractResource) GetCustomRoutes() ([]CustomRoute, error) {
	return []CustomRoute{}, nil
}

// GetRouter returns raw routes, those that don't return an attribute
func (ar *AbstractResource) GetRouter() chi.Router {
	return ar.router
}

func (ar *AbstractResource) parseURLParamValue(paramValue string) interface{} {
	parsedBool, err := strconv.ParseBool(paramValue)
	if err == nil {
		return parsedBool
	}

	parsedInt, err := strconv.ParseInt(paramValue, 10, 64)
	if err == nil {
		return parsedInt
	}

	parsedUint, err := strconv.ParseUint(paramValue, 10, 64)
	if err == nil {
		return parsedUint
	}

	parsedFloat, err := strconv.ParseFloat(paramValue, 64)
	if err == nil {
		return parsedFloat
	}

	return paramValue
}

func (ar *AbstractResource) GetURLParamValues(paramKey string, request *http.Request) []interface{} {
	paramValues, ok := request.URL.Query()[paramKey]
	if !ok || len(paramValues) == 0 {
		return nil
	}

	var values []interface{}
	for _, value := range paramValues {
		values = append(values, ar.parseURLParamValue(value))
	}

	return values
}

func (ar *AbstractResource) GetURLParamValue(paramKey string, request *http.Request) interface{} {
	paramValues, ok := request.URL.Query()[paramKey]
	if !ok || len(paramValues) == 0 {
		return nil
	}

	return ar.parseURLParamValue(paramValues[0])
}

func (ar *AbstractResource) GetURLParamBoolOrDefault(request *http.Request, paramKey string, defaultValue bool) bool {
	booleanParam, ok := ar.GetURLParamValue(paramKey, request).(bool)
	if !ok {
		return defaultValue
	}

	return booleanParam
}

func (ar *AbstractResource) GetURLParamInt64OrDefault(request *http.Request, paramKey string, defaultValue int64) int64 {
	int64Param, ok := ar.GetURLParamValue(paramKey, request).(int64)
	if !ok {
		return defaultValue
	}

	return int64Param
}

func (ar *AbstractResource) GetURLParamUint64OrDefault(request *http.Request, paramKey string, defaultValue uint64) uint64 {
	uint64Param, ok := ar.GetURLParamValue(paramKey, request).(uint64)
	if !ok {
		return defaultValue
	}

	return uint64Param
}

func (ar *AbstractResource) GetURLParamFloatOrDefault(request *http.Request, paramKey string, defaultValue float64) float64 {
	float64Param, ok := ar.GetURLParamValue(paramKey, request).(float64)
	if !ok {
		return defaultValue
	}

	return float64Param
}

func (ar *AbstractResource) GetURLParamStringOrDefault(request *http.Request, paramKey string, defaultValue string) string {
	stringParam, ok := ar.GetURLParamValue(paramKey, request).(string)
	if !ok {
		return defaultValue
	}

	return stringParam
}

// GetRouterURLParam returns the router parameters values by key. e.g.: `/endpoint/{id} -> "value of id"
func (ar *AbstractResource) GetRouterURLParam(request *http.Request, paramKey string) string {
	return chi.URLParam(request, paramKey)
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

func (ar *AbstractResource) registerCustomRoutes() error {
	CustomRoutes, _ := ar.Resource.GetCustomRoutes()

	// not all resources support custom routes
	if CustomRoutes == nil {
		return nil
	}

	// iterate through the custom routes and register a handler for them
	for _, customRoute := range CustomRoutes {
		var routerFunc func(string, http.HandlerFunc)

		switch customRoute.Method {
		case http.MethodGet:
			routerFunc = ar.router.Get
		case http.MethodPost:
			routerFunc = ar.router.Post
		case http.MethodPut:
			routerFunc = ar.router.Put
		case http.MethodDelete:
			routerFunc = ar.router.Delete
		default:
			return errors.Errorf("Invalid method %s used in custom route", customRoute.Method)
		}

		customRouteCopy := customRoute
		ar.Logger.DebugWith("Registered custom route",
			"stream", customRoute.Stream,
			"pattern", customRoute.Pattern,
			"method", customRoute.Method)

		if customRoute.Stream {
			routerFunc(customRoute.Pattern, func(responseWriter http.ResponseWriter, request *http.Request) {
				ar.callCustomStreamRouteFunc(responseWriter, request, customRouteCopy.StreamRouteFunc)
			})
		} else {
			routerFunc(customRoute.Pattern, func(responseWriter http.ResponseWriter, request *http.Request) {
				ar.callCustomRouteFunc(responseWriter, request, customRouteCopy.RouteFunc)
			})
		}
	}

	return nil
}

func (ar *AbstractResource) handleGetList(responseWriter http.ResponseWriter, request *http.Request) {
	encoder := ar.encoderFactory.NewEncoder(responseWriter, ar.name)

	allResources, err := ar.Resource.GetAll(request)

	// if the error warranted writing a response or if there are no attributes - do nothing
	if ar.writeStatusCodeAndErrorReason(responseWriter, err, http.StatusOK) {
		return
	}

	if allResources == nil {
		allResources = map[string]Attributes{}
	}

	encoder.EncodeResources(allResources)
}

func (ar *AbstractResource) handleGetDetails(responseWriter http.ResponseWriter, request *http.Request) {
	encoder := ar.encoderFactory.NewEncoder(responseWriter, ar.name)

	// registered as "/:id/"
	resourceID := ar.GetRouterURLParam(request, "id")

	// delegate to child
	attributes, err := ar.Resource.GetByID(request, resourceID)

	// if not found return 404
	if err == nil && attributes == nil {
		responseWriter.WriteHeader(http.StatusNotFound)
		return
	}

	// if the error warranted writing a response or if there are no attributes - do nothing
	if ar.writeStatusCodeAndErrorReason(responseWriter, err, http.StatusOK) {
		return
	}

	if attributes == nil {
		attributes = Attributes{}
	}

	encoder.EncodeResource(resourceID, attributes)
}

func (ar *AbstractResource) handleCreate(responseWriter http.ResponseWriter, request *http.Request) {
	encoder := ar.encoderFactory.NewEncoder(responseWriter, ar.name)

	// delegate to child
	resourceID, attributes, err := ar.Resource.Create(request)

	defaultStatusCode := http.StatusCreated
	if attributes == nil {
		defaultStatusCode = http.StatusNoContent
	}

	// if the error warranted writing a response or if there are no attributes - do nothing
	if ar.writeStatusCodeAndErrorReason(responseWriter, err, defaultStatusCode) || attributes == nil {
		return
	}

	encoder.EncodeResource(resourceID, attributes)
}

func (ar *AbstractResource) handleUpdate(responseWriter http.ResponseWriter, request *http.Request) {
	encoder := ar.encoderFactory.NewEncoder(responseWriter, ar.name)

	// registered as "/:id/"
	resourceID := ar.GetRouterURLParam(request, "id")

	// delegate to child
	attributes, err := ar.Resource.Update(request, resourceID)

	defaultStatusCode := http.StatusOK
	if attributes == nil {
		defaultStatusCode = http.StatusNoContent
	}

	// if the error warranted writing a response or if there are no attributes - do nothing
	if ar.writeStatusCodeAndErrorReason(responseWriter, err, defaultStatusCode) || attributes == nil {
		return
	}

	encoder.EncodeResource(resourceID, attributes)
}

func (ar *AbstractResource) handleDelete(responseWriter http.ResponseWriter, request *http.Request) {

	// registered as "/:id/"
	resourceID := ar.GetRouterURLParam(request, "id")

	// delegate to child
	err := ar.Resource.Delete(request, resourceID)

	// get the status code from the error
	ar.writeStatusCodeAndErrorReason(responseWriter, err, http.StatusNoContent)
}

func (ar *AbstractResource) callCustomStreamRouteFunc(responseWriter http.ResponseWriter,
	request *http.Request,
	routeFunc CustomRouteFuncStream) {

	response, err := routeFunc(request)

	// close at last
	defer func() {
		if response != nil && response.ReadCloser != nil {
			response.ReadCloser.Close()
		}
	}()

	if err != nil || response == nil {
		ar.Logger.WarnWithCtx(request.Context(),
			"Custom routed handler failed",
			"response", response)
		ar.writeStatusCodeAndErrorReason(responseWriter, err, http.StatusInternalServerError)
		return
	}

	// set response headers
	for headerKey, headerValue := range response.Headers {
		responseWriter.Header().Set(headerKey, headerValue)
	}

	// set response status code
	responseWriter.WriteHeader(response.StatusCode)

	// whether to force-ly flush data
	if response.ForceFlush {
		if response.FlushInternal == 0 {
			response.FlushInternal = time.Second
		}

		// HTTP framework (go-chi package) does not re-flush automatically
		// This go routine helps out and flushing every <interval> giving the user
		// the experience of lively-streaming output
		go func() {
			defer common.CatchAndLogPanic(request.Context(), // nolint: errcheck
				ar.Logger,
				"flush-custom-stream-func")

			for {
				select {
				case <-time.After(response.FlushInternal):
					if flusher, ok := responseWriter.(http.Flusher); ok {
						flusher.Flush()
					}
				case <-request.Context().Done():
					response.ReadCloser.Close() // nolint: errcheck
					return
				}
			}
		}()
	}

	// stream
	if _, err := io.Copy(responseWriter, response.ReadCloser); err != nil && err != io.EOF {
		ar.Logger.WarnWithCtx(request.Context(), "Failed to stream", "err", err.Error())
		responseWriter.Write([]byte(err.Error())) // nolint: errcheck
	}
}

func (ar *AbstractResource) callCustomRouteFunc(responseWriter http.ResponseWriter,
	request *http.Request,
	routeFunc CustomRouteFunc) {

	// see if the resource only supports a single record
	response, err := routeFunc(request)

	if err != nil {
		ar.Logger.WarnWith("Custom routed handler failed",
			"err", err,
			"routeFunc", routeFunc,
			"request", request)
	}

	// if response object was not created, fill a placeholder
	if response == nil {
		response = &CustomRouteFuncResponse{
			Single:     true,
			StatusCode: http.StatusInternalServerError,
			Headers:    map[string]string{"Content-Type": "application/json"},
		}
		ar.Logger.WarnWith("Response object not filled by handler, using placeholder",
			"routeFunc", routeFunc,
			"request", request,
			"response", response)
	}

	// set headers in response
	for headerKey, headerValue := range response.Headers {
		responseWriter.Header().Set(headerKey, headerValue)
	}

	// if the error warranted writing a response or if there are no attributes - do nothing
	if ar.writeStatusCodeAndErrorReason(responseWriter, err, response.StatusCode) {
		return
	}

	if response.Resources == nil {

		switch response.StatusCode {
		case http.StatusNoContent:

			// nothing to do
			break
		default:

			// write a valid, empty JSON
			if _, err := responseWriter.Write([]byte("{}")); err != nil {

				// should never happen
				ar.Logger.ErrorWith("Response writer failed writing empty resources",
					"err", err,
					"routeFunc", routeFunc,
					"request", request)
			}

		}

		return
	}

	encoder := ar.encoderFactory.NewEncoder(responseWriter, response.ResourceType)

	if response.Single {

		// to get the first, we must iterate over range
		for resourceKey, resourceAttributes := range response.Resources {
			if resourceAttributes != nil {
				encoder.EncodeResource(resourceKey, resourceAttributes)
			}

			break
		}

	} else {
		encoder.EncodeResources(response.Resources)
	}
}

// returns "false" if did not write the actual response, true if it did
func (ar *AbstractResource) writeErrorReason(responseWriter io.Writer, err error) {
	if err == nil {
		return
	}

	// to hold the error
	buffer := bytes.Buffer{}

	// there can be three types of errors here:
	// 1. a basic golang error, if the user returned something like errors.New("Whatever")
	// 2. a pkg/error, if the user returned errors.Wrap(...)
	// 3. a nuclio.ErrorWithStatusCode

	// if the error is with status code, get the underlying error. otherwise, PrintErrorStack fails the type
	// assertion that ErrorWithStatusCode is of type errors.Error
	switch typedErr := err.(type) {
	case nuclio.ErrorWithStatusCode:
		err = typedErr.GetError()
	case *nuclio.ErrorWithStatusCode:
		err = typedErr.GetError()
	}

	errorCause := ""
	if errors.RootCause(err) != nil {
		errorCause = errors.RootCause(err).Error()
	}

	// try to get the error stack
	errors.PrintErrorStack(&buffer, err, 10)

	// format to json manually
	serializedError, _ := json.Marshal(struct {
		Error           string `json:"error"`
		ErrorStackTrace string `json:"errorStackTrace"`
	}{
		errorCause,
		buffer.String(),
	})

	// write to the response
	responseWriter.Write(serializedError) // nolint: errcheck
}

func (ar *AbstractResource) statusCodeIsError(statusCode int) bool {
	return statusCode >= 400
}

// write error and status code if applicable
func (ar *AbstractResource) writeStatusCodeAndErrorReason(responseWriter http.ResponseWriter,
	err error,
	defaultStatusCode int) bool {

	// get the status code from the error
	statusCode := common.ResolveErrorStatusCodeOrDefault(err, defaultStatusCode)

	// if the status code is an actual error, write the error reason and return
	if ar.statusCodeIsError(statusCode) {
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.WriteHeader(statusCode)

		ar.writeErrorReason(responseWriter, err)

		return true
	}

	responseWriter.WriteHeader(statusCode)
	return false
}
