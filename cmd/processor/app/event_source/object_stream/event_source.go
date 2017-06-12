package object_stream

import (
	"github.com/nuclio/nuclio/cmd/processor/app/event_source"
	"github.com/nuclio/nuclio/cmd/processor/app/worker"
	"github.com/nuclio/nuclio/pkg/logger"
	"github.com/iguazio/v3io"
	"fmt"
	"net/http"
)

type object_stream struct {
	event_source.DefaultEventSource
	restart bool
	url string
	containerAlias string
	paths []string
}

func NewEventSource(logger logger.Logger,
	workerAllocator worker.WorkerAllocator,
	restart bool,
	url string,
	containerAlias string,
	paths []string) (event_source.EventSource, error) {

	newEventSource := object_stream{
		DefaultEventSource: event_source.DefaultEventSource{
			Logger:          logger,
			WorkerAllocator: workerAllocator,
			Class:           "stream",
			Kind:            "object_stream",
		},
		restart: restart,
		url: url,
		containerAlias: containerAlias,
		paths: paths,
	}

	return &newEventSource, nil
}

func (obs *object_stream) Start(checkpoint event_source.Checkpoint) error {
	go obs.pollObjectEvents()

	return nil
}

func (obs *object_stream) Stop(force bool) (event_source.Checkpoint, error) {

	// TODO
	return nil, nil
}

func (obs *object_stream) pollObjectEvents() {
	url := fmt.Sprintf("%s/%s", obs.url, obs.containerAlias)

	obs.Logger.With(logger.Fields{
		"restart": obs.restart,
		"url": url,
		"paths": obs.paths,
	}).Info("Starting")

	v3ioClient := v3io.V3iow{
		Url: url,
		Tr: &http.Transport{},
		DebugState: true,
	}



	// shove all paths in a channel and bring up workers to read from this channel
	for _, path := obs.paths {

	}


}

func (obs *object_stream) onV3ioLog(formattedRecord string) {
	obs.Logger.Debug(formattedRecord)
}
