/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v3ioitempoller

import (
	"fmt"
	"strings"
	"sync"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/trigger/poller"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/nuclio-sdk"
	"github.com/v3io/v3io-go-http"
)

type v3ioItemPoller struct {
	poller.AbstractPoller
	configuration *Configuration
	query         string
	attributes    string
	firstPoll     bool
	v3ioContainer *v3io.Container
}

func newTrigger(logger nuclio.Logger,
	workerAllocator worker.Allocator,
	configuration *Configuration) (trigger.Trigger, error) {

	newTrigger := v3ioItemPoller{
		AbstractPoller: *poller.NewAbstractPoller(logger, workerAllocator, &configuration.Configuration),
		configuration:  configuration,
		firstPoll:      true,
	}

	// register self as the poller (to allow parent to call child functions)
	newTrigger.SetPoller(&newTrigger)

	// connect to v3io container
	// TODO: create context, session, container from configuration

	// populate fields required to get items
	newTrigger.attributes = newTrigger.getAttributesToRequest()
	newTrigger.query = newTrigger.getQueryToRequest()

	return &newTrigger, nil
}

func (vip *v3ioItemPoller) GetNewEvents(eventsChan chan nuclio.Event) error {

	vip.Logger.InfoWith("Getting new events", "configuration", vip.configuration)

	// initialize a wait group with the # of paths we need to get
	var itemsGetterWaitGroup sync.WaitGroup
	itemsGetterWaitGroup.Add(len(vip.configuration.Paths))

	// shove all paths in a channel and bring up workers to read from this channel
	for _, path := range vip.configuration.Paths {

		go func(path string) {

			// get changed objects from this path
			vip.getItems(path, eventsChan)

			// reduce one from the wait group
			itemsGetterWaitGroup.Done()
		}(path)
	}

	// wait for all item getters to complete
	itemsGetterWaitGroup.Wait()

	// we're done. add a "nil" into the channel to indicate where the cycle completes
	eventsChan <- nil

	// if the first poll is over, we need to re-generate our query, which may be different between
	// first poll and subsequent polls
	if vip.firstPoll {
		vip.firstPoll = false
		vip.query = vip.getQueryToRequest()
	}

	return nil
}

// handle a set of events that were processed
func (vip *v3ioItemPoller) PostProcessEvents(events []nuclio.Event, responses []interface{}, errors []error) {

	// get the sec / nsec attributes
	triggerAttributes := vip.getTriggerAttributes()
	secAttribute := triggerAttributes[0]
	nsecAttribute := triggerAttributes[1]

	// iterate over events
	for eventIdx, event := range events {

		// if processing successful
		if errors[eventIdx] == nil {

			updatedAttributes := map[string]interface{}{
				secAttribute:  int(event.GetTimestamp().Unix()),
				nsecAttribute: int(event.GetTimestamp().UnixNano()),
			}

			// update the attributes
			err := vip.v3ioContainer.Sync.UpdateItem(&v3io.UpdateItemInput{
				Path:       event.(*Event).GetPath(),
				Attributes: updatedAttributes,
			})

			if err != nil {

				// TODO: handle error somehow?
				vip.Logger.WarnWith("Failed to update item", "err", err)
			}
		}
	}
}

func (vip *v3ioItemPoller) GetConfig() map[string]interface{} {
	return common.StructureToMap(vip.configuration)
}

func (vip *v3ioItemPoller) getItems(path string,
	eventsChan chan nuclio.Event) error {
	//
	//vip.Logger.DebugWith("Getting items", "path", path)
	//
	//// to get the first page of items, the marker must be clear
	//marker := ""
	//
	//for allItemsReceived := false; !allItemsReceived; {
	//
	//	response, err := vip.v3ioContainer.GetItems(path,
	//		vip.attributes,
	//		vip.query,
	//		marker,
	//		250,
	//		vip.configuration.ShardID,
	//		vip.configuration.TotalShards)
	//
	//	if err != nil {
	//		return errors.Wrap(err, "Failed to get items")
	//	}
	//
	//	// create events from items, write them to the channel
	//	vip.createEventsFromItems(path, response.Items, eventsChan)
	//
	//	// set whether or not all items have been received
	//	allItemsReceived = response.LastItemIncluded == "TRUE"
	//
	//	// set the marker for the next request
	//	if !allItemsReceived {
	//		marker = response.NextMarker
	//	}
	//}

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

	// add the attributes the trigger adds
	attributes = append(attributes, vip.getTriggerAttributes()...)

	// add attributes requested by the user
	attributes = append(attributes, vip.configuration.Attributes...)

	vip.Logger.DebugWith("Gathered attributes to request", "attributes", attributes)

	// request format is attributes separated by comma
	return strings.Join(attributes, ",")
}

// get attributes added by the trigger
func (vip *v3ioItemPoller) getTriggerAttributes() []string {
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

	vip.Logger.DebugWith("Gathered queries to request", "queries", queries)

	// wrap each query in parenthesis
	queries = vip.encloseStrings(queries, "(", ")")

	// join all queries with an "AND" operation
	return strings.Join(queries, " and ")
}

func (vip *v3ioItemPoller) getIncrementalQuery() []string {

	// if user doesn't want incremental changes, we don't querie by mtime
	// if this is the first poll, don't filter by mtime since the objects may
	// not even have the event soure labels
	if vip.firstPoll || !vip.configuration.Incremental {
		return nil
	}

	// get the sec / nsec attributes
	triggerAttributes := vip.getTriggerAttributes()
	secAttribute := triggerAttributes[0]
	nsecAttribute := triggerAttributes[1]

	// create the query - get objects whose mtime is later than the attributes the triggers
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
		enclosedStrings = append(enclosedStrings, start+inputString+end)
	}

	return enclosedStrings
}

func (vip *v3ioItemPoller) createEventsFromItems(path string,
	items []interface{},
	eventsChan chan nuclio.Event) {

	//for _, item := range items {
	//	name := item["__name"].(string)
	//
	//	event := Event{
	//		item: &item,
	//		url:  vip.v3ioClient.URL + "/" + path + "/" + name,
	//		path: path + "/" + name,
	//	}
	//
	//	// shove event to channe
	//	eventsChan <- &event
	//}
}
