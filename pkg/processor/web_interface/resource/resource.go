package resource

import (
	"encoding/json"
	"net/http"

	"github.com/nuclio/nuclio/cmd/processor/app"
	"github.com/nuclio/nuclio/pkg/processor/web_interface"

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
	getAll() map[string]map[string]interface{}

	// return all instances for resources with single instances
	getSingle() (string, map[string]interface{})
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
	web_interface.ResourceRegistrySingleton.Register(ar.name, ar)
}

func (ar *abstractResource) registerMethodRoutes() error {
	for _, resourceMethod := range ar.resourceMethods {
		switch resourceMethod {
		case resourceMethodGetList:
			ar.router.Get("/", ar.list)
		}
	}

	return nil
}

func (ar *abstractResource) registerRoutes() error {
	return nil
}

// return all instances for resources with multiple instances
func (ar *abstractResource) getAll() map[string]map[string]interface{} {
	return nil
}

// return all instances for resources with single instances
func (ar *abstractResource) getSingle() (string, map[string]interface{}) {
	return "", nil
}

func (ar *abstractResource) list(w http.ResponseWriter, r *http.Request) {
	enc := json.NewEncoder(w)

	// see if the resource only supports a single record
	singleResourceKey, singleResourceAttributes := ar.resource.getSingle()

	if singleResourceAttributes != nil {

		enc.Encode(&jsonapiResponse{Data: jsonapiResource{
			Type:       ar.name,
			ID:         singleResourceKey,
			Attributes: singleResourceAttributes,
		}})

	} else {

		// resource supports multiple instances
		jsonapiResources := []jsonapiResource{}

		// delegate to child resource to get all
		for resourceKey, resourceAttributes := range ar.resource.getAll() {
			jsonapiResources = append(jsonapiResources, jsonapiResource{
				Type:       ar.name,
				ID:         resourceKey,
				Attributes: resourceAttributes,
			})
		}

		enc.Encode(&jsonapiResponse{Data: jsonapiResources})
	}
}
