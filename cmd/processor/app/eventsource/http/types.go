package http

import "github.com/nuclio/nuclio/cmd/processor/app/eventsource"

type Configuration struct {
	eventsource.Configuration
	ListenAddress string
}

type Response struct {
	StatusCode  int
	ContentType string
	Header      map[string]string
	Body        []byte
}
