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
	"encoding/json"
	"net/http"
)

//
// Encoder
//

type jsonAPIEncoder struct {
	jsonEncoder    *json.Encoder
	responseWriter http.ResponseWriter
	resourceType   string
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
	jae.jsonEncoder.Encode(&jsonapiResponse{Data: jsonapiResource{ // nolint: errcheck
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

	jae.jsonEncoder.Encode(&jsonapiResponse{Data: jsonapiResources}) // nolint: errcheck
}

//
// Factory
//

type JSONAPIEncoderFactory struct{}

func (jaef *JSONAPIEncoderFactory) NewEncoder(responseWriter http.ResponseWriter, resourceType string) Encoder {
	responseWriter.Header().Set("Content-Type", "application/json")

	return &jsonAPIEncoder{
		jsonEncoder:    json.NewEncoder(responseWriter),
		responseWriter: responseWriter,
		resourceType:   resourceType,
	}
}
