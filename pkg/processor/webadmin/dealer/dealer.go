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
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
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
	message := d.createReply()
	for _, trigger := range d.processor.GetTriggers() {
		tasks := d.getTasks(trigger)
		message.Triggers[trigger.GetID()] = newTrigger(tasks, len(tasks), false)
	}

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
			d.logger.InfoWith("Creating new trigger", "id", triggerID, "config", trigger)
			if err := d.createTrigger(triggerID, trigger); err != nil {
				d.writeError(w, http.StatusInternalServerError, err)
				return
			}
			continue
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
	}

	d.addMissingTasks(triggers, reply)
	d.writeReply(w, reply)
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

// writeReply write message as JSON. It'll add total events to the message
func (d *Dealer) writeReply(w http.ResponseWriter, message *Message) {
	d.addTotalEvents(message)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(message) // nolint: errcheck
}
