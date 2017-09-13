package restful

import (
	"encoding/json"
	"net/http"
)

//
// Encoder
//

type jsonAPIEncoder struct {
	jsonEncoder  *json.Encoder
	resourceType string
}

type jsonapiResponse struct {
	Data interface{} `json:"data"`
}

type jsonapiResource struct {
	ID         string     `json:"id"`
	Type       string     `json:"type"`
	Attributes Attributes `json:"attributes"`
}

// encode a single resource
func (jae *jsonAPIEncoder) EncodeResource(resourceID string, resourceAttributes Attributes) {
	jae.jsonEncoder.Encode(&jsonapiResponse{Data: jsonapiResource{
		Type:       jae.resourceType,
		ID:         resourceID,
		Attributes: resourceAttributes,
	}})
}

// encode multiple resources
func (jae *jsonAPIEncoder) EncodeResources(resources map[string]Attributes) {
	jsonapiResources := []jsonapiResource{}

	// delegate to child resource to get all
	for resourceID, resourceAttributes := range resources {
		jsonapiResources = append(jsonapiResources, jsonapiResource{
			Type:       jae.resourceType,
			ID:         resourceID,
			Attributes: resourceAttributes,
		})
	}

	jae.jsonEncoder.Encode(&jsonapiResponse{Data: jsonapiResources})
}

//
// Factory
//

type JsonAPIEncoderFactory struct{}

func (jaef *JsonAPIEncoderFactory) NewEncoder(responseWriter http.ResponseWriter, resourceType string) Encoder {
	return &jsonAPIEncoder{
		jsonEncoder:  json.NewEncoder(responseWriter),
		resourceType: resourceType,
	}
}
