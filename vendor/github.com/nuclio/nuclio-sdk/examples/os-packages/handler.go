// Currently, if processor.yaml is not provided, nubuild will look for
// a Handler() function in the handler package. Future implementations
// will alleviate this limitation

package handler

import (
	"github.com/nuclio/nuclio-sdk"
)

func Handler(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
	context.Logger.Info("Event received (packages)")

	return nuclio.Response{
		StatusCode:  200,
		ContentType: "application/text",
		Body:        []byte("Response from handler"),
	}, nil
}
