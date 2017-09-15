package incrementor

import (
	"github.com/nuclio/nuclio-sdk"
)

func Increment(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
	incrementedBody := []byte{}

	context.Logger.InfoWith("Incrementing body", "body", string(event.GetBody()))

	for _, byteValue := range event.GetBody() {
		incrementedBody = append(incrementedBody, byteValue + 1)
	}

	return incrementedBody, nil
}
