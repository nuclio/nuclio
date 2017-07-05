package v3ioitempoller

import "github.com/nuclio/nuclio/cmd/processor/app/event_source/poller"

type Configuration struct {
	poller.Configuration
	Restart        bool
	URL            string
	ContainerID    int
	ContainerAlias string
	Paths          []string
	Attributes     []string
	Queries        []string
	Suffixes       []string
	Incremental    bool
	ShardID        int
	TotalShards    int
}
