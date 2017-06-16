package v3io_item_poller

import "github.com/nuclio/nuclio/cmd/processor/app/event_source/poller"

type Configuration struct {
	poller.Configuration
	Restart        bool
	URL            string
	ContainerID    int
	ContainerAlias string
	Paths          []string
}
