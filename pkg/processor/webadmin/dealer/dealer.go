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

/*
Package dealer provies HTTP API to change triggers/partition

In the dealter terminology we have "tasks" which currently map to paritions but
can mean other things in the future.
*/
package dealer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"time"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/trigger/partitioned"

	"github.com/nuclio/logger"
)

// Processor interface (to avoice cyclic import of app/processor)
type Processor interface {
	GetConfiguration() *processor.Configuration
	GetTriggers() map[string]trigger.Trigger
	CreateTrigger(triggerName string, processorConfiguration *processor.Configuration, triggerConfiguration functionconfig.Trigger) error
	RemoveTrigger(triggerID string) (functionconfig.Checkpoint, error)
}

// Dealer is handler dealer API
// We can't have dealer as restful.Resource since returned JSON structure isn't
// compatible with map[string]restful.Attributes
type Dealer struct {
	logger    logger.Logger
	processor Processor
	dealerURL string
	Host      string
	IP        string
	Port      int
}

// New returns a new dealer
func New(parentLogger logger.Logger, processor interface{}, configuration *platformconfig.WebServer) (*Dealer, error) {
	processorInstance, ok := processor.(Processor)
	if !ok {
		return nil, errors.Errorf("bad processor (type=%T)", processor)
	}

	dealer := &Dealer{
		logger:    parentLogger.GetChild("dealer"),
		processor: processorInstance,
		dealerURL: processorInstance.GetConfiguration().Spec.DealerURL,
	}
	dealer.IP = dealer.getIP()
	dealer.Host = dealer.getHost()
	dealer.Port = dealer.getPort(configuration)
	return dealer, nil
}

// Get handles GET request
func (d *Dealer) Get(w http.ResponseWriter, r *http.Request) {
	message := d.currentStatus()
	d.writeReply(w, message)
}

// Post handles POST request
// TODO: This is long, break to smaller parts
func (d *Dealer) Post(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	dealerRequest := Message{}
	if err := decoder.Decode(&dealerRequest); err != nil {
		d.writeError(w, http.StatusBadRequest, err)
		return
	}

	if dealerRequest.DealerURL != "" {
		d.dealerURL = dealerRequest.DealerURL
	}

	reply := d.createReply()
	triggers := d.processor.GetTriggers()
	for triggerID, trigger := range dealerRequest.Triggers {
		triggerInstance, triggerFound := triggers[triggerID]

		// Create new trigger
		if !triggerFound && !trigger.Disable {
			d.logger.ErrorWith("New trigger not supported", "id", triggerID, "config", trigger)
			err := fmt.Errorf("New trigger (%s) - not supported", triggerID)
			d.writeError(w, http.StatusInternalServerError, err)
			return
		}

		if !triggerFound {
			err := errors.Errorf("unknown trigger - %q", triggerID)
			d.writeError(w, http.StatusBadRequest, err)
			return
		}

		stream, isStream := triggerInstance.(partitioned.Stream)
		if !isStream {
			d.logger.WarnWith("trigger is not partitioned", "id", triggerID)
			continue
		}

		triggerConfig := d.processor.GetConfiguration().Spec.Triggers[triggerID]
		if attribute := d.checkUnsupported(trigger, triggerConfig); attribute != "" {
			err := fmt.Errorf("Unsupported change to %q in trigger %q", attribute, triggerID)
			d.writeError(w, http.StatusBadRequest, err)
			return
		}

		// Stop trigger
		if trigger.Disable {
			d.logger.InfoWith("Disabling trigger", "id", triggerID)
			triggerCheckpoint, err := d.processor.RemoveTrigger(triggerID)
			if err != nil {
				httpError := errors.Wrapf(err, "Can't stop trigger %v", triggerID)
				d.writeError(w, http.StatusBadRequest, httpError)
				return
			}

			// We add this task to the reply since it won't be in the processor
			// triggers anymore
			tasks := d.triggerCheckpointToTasks(triggerCheckpoint)
			reply.Triggers[triggerID] = newTrigger(tasks, len(tasks), true)
			continue
		}

		partitions := stream.GetPartitions()
		var deletedTasks []Task
		for _, task := range trigger.Tasks {
			partitionConfig := &functionconfig.Partition{
				Checkpoint: &task.Checkpoint,
				ID:         fmt.Sprintf("%d", task.ID),
			}

			_, partitionFound := partitions[task.ID]

			// Create new partition
			if !partitionFound && d.isRunState(task.State) {
				d.logger.InfoWith("Adding partition", "trigger", triggerID, "config", partitionConfig)
				if err := triggerInstance.AddPartition(partitionConfig); err != nil {
					d.writeError(w, http.StatusInternalServerError, err)
					return
				}
				continue
			}

			if !partitionFound {
				err := errors.Errorf("Partition %v not found in trigger %v", task.ID, triggerID)
				d.writeError(w, http.StatusBadRequest, err)
				return
			}

			if d.isRunState(task.State) {
				d.logger.InfoWith("Partition already running", "id", task.ID)
				// TODO: Do we want to support seeking to another checkpoint?
				continue
			}

			if !d.isStopState(task.State) {
				err := errors.Errorf("Trigger %v, Task %v - unknown action %v", triggerID, task.ID, task.State)
				d.writeError(w, http.StatusBadRequest, err)
				return
			}

			d.logger.InfoWith("Removing task", "trigger", triggerID, "task", task.ID)
			checkpoint, err := triggerInstance.RemovePartition(partitionConfig)
			if err != nil {
				httpError := errors.Wrapf(err, "Can't delete task %v from trigger %v", task.ID, triggerID)
				d.writeError(w, http.StatusInternalServerError, httpError)
				return
			}

			deletedTask := Task{
				ID:         task.ID,
				Checkpoint: d.checkpointToStr(checkpoint),
				State:      TaskStateDeleted,
			}

			deletedTasks = append(deletedTasks, deletedTask)
		}

		tasks := d.streamTasks(stream)

		tasks = append(tasks, deletedTasks...)
		reply.Triggers[triggerID] = newTrigger(tasks, len(tasks)-len(deletedTasks), false)
		if err := triggerInstance.GetAllocator().GC(); err != nil {
			d.logger.WarnWith("Can't run allocator GC", "error", err)
		}
	}

	d.addMissingTasks(triggers, reply)
	d.writeReply(w, reply)
}

// Shutdown dealer
func (d *Dealer) Shutdown() {
	if d.dealerURL == "" {
		return
	}

	message := d.currentStatus()
	var buf bytes.Buffer

	if err := json.NewEncoder(&buf).Encode(message); err != nil {
		d.logger.WarnWith("Can't create shutdown message", "error", err)
		return
	}

	resp, err := http.Post(d.dealerURL, "application/json", &buf)
	if err != nil {
		d.logger.WarnWith("Can't send shutdown message to dealer", "error", err, "url", d.dealerURL)
		return
	}

	if resp.StatusCode != http.StatusOK {
		d.logger.WarnWith("Can't error sending shutdown message to dealer", "error", resp.Status, "url", d.dealerURL, "code", resp.StatusCode)
		return
	}
}

func (d *Dealer) currentStatus() *Message {
	message := d.createReply()
	for _, trigger := range d.processor.GetTriggers() {
		tasks := d.getTasks(trigger)
		message.Triggers[trigger.GetID()] = newTrigger(tasks, len(tasks), false)
	}

	return message
}

func (d *Dealer) createReply() *Message {
	config := d.processor.GetConfiguration()

	return &Message{
		Name:        d.Host,
		Namespace:   config.Meta.Namespace,
		Function:    config.Meta.Name,
		Version:     fmt.Sprintf("%d", config.Spec.Version),
		Alias:       config.Spec.Alias,
		IP:          d.IP,
		Port:        d.Port,
		State:       0,
		TotalEvents: 0,
		Timestamp:   time.Now(),
		DealerURL:   d.dealerURL,
		Triggers:    make(map[string]*Trigger),
	}

}

func (d *Dealer) getTasks(trigger trigger.Trigger) []Task {
	stream, ok := trigger.(partitioned.Stream)
	if !ok {
		return nil
	}

	partitions := stream.GetPartitions()
	tasks := make([]Task, 0, len(partitions))
	for _, partition := range partitions {
		task := Task{
			ID:         partition.GetID(),
			State:      TaskStateRunning,
			Checkpoint: d.checkpointToStr(partition.GetCheckpoint()),
		}
		tasks = append(tasks, task)
	}

	return tasks
}

func (d *Dealer) getPort(config *platformconfig.WebServer) int {
	_, portString, err := net.SplitHostPort(config.ListenAddress)
	if err != nil {
		d.logger.WarnWith("Can't parse port", "error", err, "address", config.ListenAddress)
		return -1
	}

	port, err := strconv.Atoi(portString)
	if err != nil {
		d.logger.WarnWith("Can't parse port", "error", err, "port", portString)
		return -1
	}

	return port
}

func (d *Dealer) addMissingTasks(triggers map[string]trigger.Trigger, reply *Message) {
	for triggerID, trigger := range triggers {
		if _, ok := reply.Triggers[triggerID]; ok {
			continue
		}

		var tasks []Task
		stream, isStream := trigger.(partitioned.Stream)
		if isStream {
			tasks = d.streamTasks(stream)
		}
		reply.Triggers[triggerID] = newTrigger(tasks, len(tasks), false)
	}
}

func (d *Dealer) createTrigger(triggerID string, trigger *Trigger) error {
	processorConfig := d.processor.GetConfiguration()
	triggerConfig, ok := processorConfig.Spec.Triggers[triggerID]

	// TODO: Support new triggers on-the-fly
	if !ok {
		return errors.Errorf("Can't find configuration for trigger %s", triggerID)
	}

	if err := d.processor.CreateTrigger(triggerID, processorConfig, triggerConfig); err != nil {
		return errors.Wrap(err, "Can't create trigger")
	}

	return nil
}

func (d *Dealer) checkpointToStr(checkpoint functionconfig.Checkpoint) string {
	if checkpoint == nil {
		return emptyCheckpoint
	}

	return *checkpoint
}

// TODO: Not happy with calling 8.8.8.8 to get IP
func (d *Dealer) getIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:53")
	if err != nil {
		d.logger.WarnWith("Can't find IP", "error", err)
		return ""
	}

	defer conn.Close() // nolint: errcheck

	localAddr := conn.LocalAddr()
	ip, _, err := net.SplitHostPort(localAddr.String())
	if err != nil {
		return ""
	}

	return ip
}

func (d *Dealer) writeError(w http.ResponseWriter, status int, err error) {
	encoder := json.NewEncoder(w)
	if status == http.StatusInternalServerError {
		d.logger.ErrorWith("HTTP Error", "error", err, "status", status)
	} else {
		d.logger.WarnWith("HTTP Error", "error", err, "status", status)
	}

	w.WriteHeader(status)
	w.Header().Set("Content-Type", "application/json")
	encoder.Encode(map[string]string{ // nolint: errcheck
		"error": err.Error(),
	})
}

func (d *Dealer) triggerCheckpointToTasks(triggerCheckpoint functionconfig.Checkpoint) []Task {
	if triggerCheckpoint == nil {
		return nil
	}

	checkpoints := make(map[int]functionconfig.Checkpoint)
	if err := json.Unmarshal([]byte(*triggerCheckpoint), &checkpoints); err != nil {
		d.logger.WarnWith("Can't decode trigger checkpoints", "error", err)
		return nil
	}

	tasks := make([]Task, 0, len(checkpoints))
	for id, checkpoint := range checkpoints {
		task := Task{
			ID:         id,
			State:      TaskStateDeleted,
			Checkpoint: d.checkpointToStr(checkpoint),
		}
		tasks = append(tasks, task)
	}

	return tasks
}

func (d *Dealer) streamTasks(stream partitioned.Stream) []Task {
	tasks := make([]Task, 0, len(stream.GetPartitions()))

	for partitionID, partition := range stream.GetPartitions() {
		task := Task{
			ID:         partitionID,
			State:      TaskStateRunning,
			Checkpoint: d.checkpointToStr(partition.GetCheckpoint()),
		}
		tasks = append(tasks, task)
	}

	return tasks
}

func (d *Dealer) addTotalEvents(message *Message) {
	for _, trigger := range d.processor.GetTriggers() {
		stats := trigger.GetStatistics()
		message.TotalEvents += stats.EventsHandleSuccessTotal + stats.EventsHandleFailureTotal
	}
}

func (d *Dealer) isRunState(taskState TaskState) bool {
	return taskState == TaskStateRunning || taskState == TaskStateAlloc
}

func (d *Dealer) isStopState(taskState TaskState) bool {
	switch taskState {
	// TODO: TaskStateUnassigned ?
	case TaskStateDeleted, TaskStateStopping:
		return true
	}

	return false
}

func (d *Dealer) getHost() string {
	host, err := os.Hostname()
	if err != nil {
		d.logger.WarnWith("Can't get host name", "error", err)
		return ""
	}

	return host
}

func (d *Dealer) finalizeMessage(message *Message) {
	d.addTotalEvents(message)
	d.addTriggersConfiguration(message)
}

// writeReply write message as JSON. It'll add some additional information
func (d *Dealer) writeReply(w http.ResponseWriter, message *Message) {
	d.finalizeMessage(message)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(message) // nolint: errcheck
}

func (d *Dealer) addTriggersConfiguration(message *Message) {
	for triggerName, triggerConfig := range d.processor.GetConfiguration().Spec.Triggers {
		trigger := message.Triggers[triggerName]
		if trigger == nil {
			d.logger.WarnWith("Unknown trigger", "name", triggerName)
			continue
		}

		trigger.Class = triggerConfig.Class
		trigger.Kind = triggerConfig.Kind
		trigger.URL = triggerConfig.URL
		trigger.Paths = make([]string, len(triggerConfig.Paths))
		copy(trigger.Paths, triggerConfig.Paths)

		trigger.Annotations = make(map[string]string)
		for key, value := range triggerConfig.Annotations {
			trigger.Annotations[key] = value
		}

		trigger.MaxTaskAllocation = triggerConfig.MaxTaskAllocation
		trigger.Attributes = make(map[string]interface{})
		for key, value := range triggerConfig.Attributes {
			trigger.Attributes[key] = value
		}
	}
}

func (d *Dealer) checkUnsupported(trigger *Trigger, triggerConfig functionconfig.Trigger) string {

	if trigger.Class != "" && trigger.Class != triggerConfig.Class {
		return "class"
	}

	if trigger.Kind != "" && trigger.Kind != triggerConfig.Kind {
		return "kind"
	}

	if trigger.URL != "" && trigger.URL != triggerConfig.URL {
		return "url"
	}

	if trigger.Paths != nil && !d.comparePaths(trigger.Paths, triggerConfig.Paths) {
		return "paths"
	}

	if trigger.Annotations != nil && !reflect.DeepEqual(trigger.Annotations, triggerConfig.Annotations) {
		return "annotations"
	}

	if trigger.Attributes != nil && !reflect.DeepEqual(trigger.Attributes, triggerConfig.Attributes) {
		return "attributes"
	}

	return ""
}

func (d *Dealer) comparePaths(paths1 []string, paths2 []string) bool {
	switch {
	case len(paths1) != len(paths2):
		return false
	case len(paths1) > len(paths2):
		paths1, paths2 = paths2, paths1
	}

	map1 := make(map[string]bool)
	for _, value := range paths1 {
		map1[value] = true
	}

	for _, value := range paths2 {
		if !map1[value] {
			return false
		}
	}

	return true
}
