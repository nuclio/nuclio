package main

import (
	"fmt"

	"github.com/nuclio/nuclio-sdk-go"
)

func Handler(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
	return context.UserData.(string), nil
}

func InitContext(context *nuclio.Context) error {
	context.UserData = fmt.Sprintf("User data initialized from context: %d", context.WorkerID)

	return nil
}
