package example

import (
    "github.com/nuclio/nuclio-sdk"
)

func GolangExample(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
    context.Logger.InfoWith("Got event",
        "url", event.GetURL(),
        "size", event.GetSize(),
        "timestamp", event.GetTimestamp())

    return nuclio.Response{
        StatusCode:  201,
        ContentType: "application/text",
        Headers: map[string]string{
            "x-v3io-something": "30",
        },
        Body: []byte("Response from golang"),
    }, nil
}
