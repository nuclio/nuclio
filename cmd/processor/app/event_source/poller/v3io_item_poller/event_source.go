package v3io_item_poller

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

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
	query         string
	attributes    string
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

	// populate fields required to get items
	newEventSource.attributes = newEventSource.getAttributesToRequest()
	newEventSource.query = newEventSource.getQueryToRequest()

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
	url := fmt.Sprintf("%s/%d", vip.configuration.URL, vip.configuration.ContainerID)

	vip.Logger.With(logger.Fields{
		"url": url,
	}).Debug("Creating v3io client")

	return &v3io.V3iow{
		Url:        url,
		Tr:         &http.Transport{},
		DebugState: true,
	}
}

func (vip *v3ioItemPoller) getItems(path string,
	eventsChan chan event.Event) error {

	vip.Logger.With(logger.Fields{
		"path": path,
	}).Debug("Getting items")

	response, err := vip.v3ioClient.GetItems(path,
		vip.attributes,
		vip.query,
		"",
		256, // TODO: handle pagination
		vip.configuration.ShardID,
		vip.configuration.TotalShards)

	if err != nil {
		return vip.Logger.Report(err, "Failed to get items")
	}

	fmt.Println(response)

	return nil
}

func (vip *v3ioItemPoller) getAttributesToRequest() string {

	attributes := []string{
		"__name",
		"__mtime_secs",
		"__mtime_nsecs",
		"__obj_type",
		"__size",
	}

	// add the attributes the event source adds
	attributes = append(attributes, vip.getEventSourceAttributes()...)

	// add attributes requested by the user
	attributes = append(attributes, vip.configuration.Attributes...)

	vip.Logger.With(logger.Fields{
		"attributes": attributes,
	}).Debug("Gathered attributes to request")

	// request format is attributes separated by comma
	return strings.Join(attributes, ",")
}

// get attributes added by the event source
func (vip *v3ioItemPoller) getEventSourceAttributes() []string {
	prefix := "__nuclio_vip_" + vip.configuration.ID

	// order is assumed (secs first, nsecs second) - do not reorder
	return []string{
		prefix + "_secs",
		prefix + "_nsecs",
	}
}

func (vip *v3ioItemPoller) getQueryToRequest() string {
	var queries []string

	// add the querie that will get only objects that change
	queries = append(queries, vip.getIncrementalQuery()...)

	// add the suffix querie
	queries = append(queries, vip.getSuffixQuery()...)

	// add the user queries
	queries = append(queries, vip.configuration.Queries...)

	vip.Logger.With(logger.Fields{
		"queries": queries,
	}).Debug("Gathered queries to request")

	// wrap each query in parenthesis
	queries = vip.encloseStrings(queries, "(", ")")

	// join all queries with an "AND" operation
	return strings.Join(queries, " and ")
}

func (vip *v3ioItemPoller) getIncrementalQuery() []string {

	// if user doesn't want incremental changes, we don't querie by mtime
	if !vip.configuration.Incremental {
		return nil
	}

	// get the sec / nsec attributes
	eventSourceAttributes := vip.getEventSourceAttributes()
	secAttribute := eventSourceAttributes[0]
	nsecAttribute := eventSourceAttributes[1]

	// create the query - get objects whose mtime is later than the attributes the event sources
	// slaps on them during post processing
	return []string{
		fmt.Sprintf("__mtime_secs > %s or (__mtime == %s and __mtime_nsecs > %s)",
			secAttribute,
			secAttribute,
			nsecAttribute),
	}
}

func (vip *v3ioItemPoller) getSuffixQuery() []string {
	var suffixQueries []string

	for _, suffix := range vip.configuration.Suffixes {
		suffixQueries = append(suffixQueries, fmt.Sprintf("ends(__name, '%s')", suffix))
	}

	if len(suffixQueries) == 0 {
		return suffixQueries
	}

	// join all the suffixes with "OR"
	return []string{
		strings.Join(suffixQueries, " or "),
	}
}

func (vip *v3ioItemPoller) encloseStrings(inputStrings []string, start string, end string) []string {
	var enclosedStrings []string

	for _, inputString := range inputStrings {
		enclosedStrings = append(enclosedStrings, start + inputString + end)
	}

	return enclosedStrings
}
