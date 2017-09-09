package resource

import (
	"encoding/json"
	"net/http"

	"github.com/nuclio/nuclio/cmd/processor/app"
	"github.com/nuclio/nuclio/pkg/processor/webadmin"

	"github.com/go-chi/chi"
	"github.com/nuclio/nuclio-sdk"
)

type jsonapiResponse struct {
	Data interface{} `json:"data"`
}

type jsonapiResource struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Attributes map[string]interface{} `json:"attributes"`
}

type resource interface {
	registerRoutes() error

	// return all instances for resources with multiple instances
	getAll(request *http.Request) map[string]map[string]interface{}

	// return all instances for resources with single instances
	getSingle(request *http.Request) (string, map[string]interface{})

	// return specific instance by ID
	getByID(request *http.Request, id string) map[string]interface{}
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
	ar.registerMethodRoutes()

	// register custom routes
	ar.resource.registerRoutes()

	return ar.router, nil
}

func (ar *abstractResource) register() {
	webadmin.ResourceRegistrySingleton.Register(ar.name, ar)
}

func (ar *abstractResource) registerMethodRoutes() error {
	for _, resourceMethod := range ar.resourceMethods {
		switch resourceMethod {
		case resourceMethodGetList:
			ar.router.Get("/", ar.list)
		case resourceMethodGetDetail:
			ar.router.Route("/{id}", func(r chi.Router) {
				r.Get("/", ar.detail)
			})
		}
	}

	return nil
}

func (ar *abstractResource) registerRoutes() error {
	return nil
}

// return all instances for resources with multiple instances
func (ar *abstractResource) getAll(request *http.Request) map[string]map[string]interface{} {
	return nil
}

// return all instances for resources with single instances
func (ar *abstractResource) getSingle(request *http.Request) (string, map[string]interface{}) {
	return "", nil
}

// return specific instance by ID
func (ar *abstractResource) getByID(request *http.Request, id string) map[string]interface{} {
	return nil
}

func (ar *abstractResource) list(responseWriter http.ResponseWriter, request *http.Request) {
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

func (ar *abstractResource) detail(responseWriter http.ResponseWriter, request *http.Request) {
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
