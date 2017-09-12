package restful

import (
	"net/http"
	"encoding/json"
)

//
// Encoder
//

type jsonEncoder struct {
	jsonEncoder *json.Encoder
	resourceType string
}

// encode a single resource
func (je *jsonEncoder) EncodeResource(resourceID string, resourceAttributes Attributes) {
	resourceAttributes["id"] = resourceID

	je.jsonEncoder.Encode(&resourceAttributes)
}

// encode multiple resources
func (je *jsonEncoder) EncodeResources(resources map[string]Attributes) {

	je.jsonEncoder.Encode(&resources)
}

//
// Factory
//

type JsonEncoderFactory struct {}

func (jef *JsonEncoderFactory) NewEncoder(responseWriter http.ResponseWriter, resourceType string) Encoder {
	return &jsonEncoder{
		jsonEncoder: json.NewEncoder(responseWriter),
		resourceType: resourceType,
	}
}
