package restful

import "net/http"

type Encoder interface {

	// encode a single resource
	EncodeResource(string, Attributes)

	// encode multiple resources
	EncodeResources(map[string]Attributes)
}

type EncoderFactory interface {

	// create an encoder
	NewEncoder(http.ResponseWriter, string) Encoder
}
