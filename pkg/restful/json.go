package restful

import (
	"encoding/json"
	"net/http"
)

//
// Encoder
//

type jsonEncoder struct {
	jsonEncoder  *json.Encoder
	resourceType string
}

// encode a single resource
func (je *jsonEncoder) EncodeResource(resourceID string, resourceAttributes Attributes) {
	resourceAttributes["id"] = resourceID

	je.jsonEncoder.Encode(&resourceAttributes)
}

// encode multiple resources
func (je *jsonEncoder) EncodeResources(resources map[string]Attributes) {
	resourceIDList := []string{}

	// if attributes is nil, we return a list
	for resourceID, resourceAttributes := range resources {

		// if there's attributes, don't return as a list
		if resourceAttributes != nil {
			break
		}

		resourceIDList = append(resourceIDList, resourceID)
	}

	// if we populated a list, return it as a simple list, otherwise as a map
	if len(resourceIDList) != 0 {
		je.jsonEncoder.Encode(&resourceIDList)
	} else {
		je.jsonEncoder.Encode(&resources)
	}
}

//
// Factory
//

type JsonEncoderFactory struct{}

func (jef *JsonEncoderFactory) NewEncoder(responseWriter http.ResponseWriter, resourceType string) Encoder {
	return &jsonEncoder{
		jsonEncoder:  json.NewEncoder(responseWriter),
		resourceType: resourceType,
	}
}
