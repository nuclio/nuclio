package v3io_item_poller

import (
	"net/http"
	"sync"
	"fmt"

	"github.com/iguazio/v3io"
	"github.com/nuclio/nuclio/cmd/processor/app/event"
	"github.com/nuclio/nuclio/cmd/processor/app/event_source"
	"github.com/nuclio/nuclio/cmd/processor/app/event_source/poller"
	"github.com/nuclio/nuclio/cmd/processor/app/worker"
	"github.com/nuclio/nuclio/pkg/logger"
)

type v3ioItemPoller struct {
	poller.AbstractPoller
	configuration *Configuration
	v3ioClient    *v3io.V3iow
}

func newEventSource(logger logger.Logger,
	workerAllocator worker.WorkerAllocator,
	configuration *Configuration) (event_source.EventSource, error) {

	newEventSource := v3ioItemPoller{
		AbstractPoller: *poller.NewAbstractPoller(logger, workerAllocator, &configuration.Configuration),
		configuration:  configuration,
	}

	// register self as the poller (to allow parent to call child functions)
	newEventSource.SetPoller(&newEventSource)

	// create a v3io client
	newEventSource.v3ioClient = newEventSource.createV3ioClient()

	return &newEventSource, nil
}

func (vip *v3ioItemPoller) GetNewEvents(eventsChan chan event.Event) error {

	vip.Logger.With(logger.Fields{
		"configuration": vip.configuration,
	}).Info("Getting new events")

	// create a query for the paths
	// query = vip.createQuery()

	// initialize a wait group with the # of paths we need to get
	var itemsGetterWaitGroup sync.WaitGroup
	itemsGetterWaitGroup.Add(len(vip.configuration.Paths))

	// shove all paths in a channel and bring up workers to read from this channel
	for _, path := range vip.configuration.Paths {

		go func() {

			// get changed objects from this path
			vip.getItems(path, eventsChan)

			// reduce one from the wait group
			itemsGetterWaitGroup.Done()
		}()
	}

	// wait for all item getters to complete
	itemsGetterWaitGroup.Wait()

	// we're done. add a "nil" into the channel to indicate where the cycle completes
	eventsChan <- nil

	return nil
}

func (vip *v3ioItemPoller) createV3ioClient() *v3io.V3iow {
	url := fmt.Sprintf("%s/%s", vip.configuration.URL, vip.configuration.ContainerAlias)

	return &v3io.V3iow{
		Url:        url,
		Tr:         &http.Transport{},
		DebugState: true,
	}
}

func (vip *v3ioItemPoller) getItems(path string,
	eventsChan chan event.Event) {

	vip.Logger.With(logger.Fields{
		"path": path,
	}).Debug("Getting items")
}
