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

package dealer

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/trigger/partitioned"
	"github.com/nuclio/nuclio/pkg/processor/util"
	"github.com/nuclio/nuclio/pkg/processor/util/updater"

	"github.com/nuclio/logger"
)

var (
	emptyCheckpoint = ""
)

// TaskState is task state
type TaskState int8

// Possible task states
const (
	TaskStateUnassigned TaskState = iota
	TaskStateRunning
	TaskStateStopping
	TaskStateDeleted
	TaskStateAlloc
	TaskStateCompleted
)

// Processor interface (to avoice cyclic import with app.processor)
// TODO: Unite with the interface in pkg/processor/util/updater/updater.go
type Processor interface {
	GetConfiguration() *processor.Configuration
	SetConfiguration(config *processor.Configuration) error
	GetTriggers() []trigger.Trigger
	GetLastUpdate() *updater.Updater
}

// Dealer is handler dealer API
// We can't have dealer as restful.Resource since returned JSON structure isn't
// compatible with map[string]restful.Attributes
type Dealer struct {
	logger    logger.Logger
	processor Processor
	Host      string
	Port      int
}

// Task is a dealer task (trigger partition)
type Task struct {
	ID         int       `json:"id"`
	State      TaskState `json:"state"`
	Checkpoint string    `json:"checkpoint"`
}

// Job is a partition/shard
type Job struct {
	TotalTasks int    `json:"totalTasks"`
	Tasks      []Task `json:"tasks"`
}

// Message is dealer request/response
type Message struct {
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	Function    string `json:"function"`
	Version     string `json:"version,omitempty"`
	Alias       string `json:"alias,omitempty"`
	IP          string `json:"ip"`
	Port        int    `json:"port"`
	State       int    `json:"state"`
	TotalEvents uint64 `json:"total_events"`
	DealerURL   string `json:"dealer_url"`

	Jobs map[string]*Job `json:"jobs"`
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
	}

	var err error
	dealer.Host, err = os.Hostname()
	if err != nil {
		dealer.logger.WarnWith("Can't get host name", "error", err)
		dealer.Host = ""
	}

	dealer.Port, err = dealer.getPort(configuration)
	if err != nil {
		dealer.logger.WarnWith("Can't parse port", "error", err, "config", configuration)
		dealer.Port = -1
	}

	return dealer, nil
}

// Get handles GET request
func (d *Dealer) Get(w http.ResponseWriter, r *http.Request) {
	message := d.createReply()

	encoder := json.NewEncoder(w)
	encoder.Encode(message)
}

func (d *Dealer) createReply() *Message {
	config := d.processor.GetConfiguration()

	message := &Message{
		Name:        config.Meta.Name,
		Namespace:   config.Meta.Namespace,
		Function:    config.Spec.Handler,
		Version:     fmt.Sprintf("%d", config.Spec.Version),
		Alias:       config.Spec.Alias,
		IP:          d.Host,
		Port:        d.Port,
		State:       0,
		TotalEvents: 0,
		DealerURL:   "",
		Jobs:        make(map[string]*Job),
	}

	for _, trigger := range d.processor.GetTriggers() {
		message.Jobs[trigger.GetID()] = &Job{
			Tasks: d.getTasks(trigger),
		}
		stats := trigger.GetStatistics()
		message.TotalEvents += stats.EventsHandleSuccessTotal + stats.EventsHandleFailureTotal
	}

	return message
}

func (d *Dealer) getTasks(trigger trigger.Trigger) []Task {
	stream, ok := trigger.(partitioned.Stream)
	if !ok {
		return nil
	}

	partitions := stream.GetPartitions()
	tasks := make([]Task, len(partitions))
	for i, partition := range partitions {
		tasks[i] = Task{
			ID:         partition.GetID(),
			State:      TaskStateRunning,
			Checkpoint: d.checkpointToStr(partition.GetCheckpoint()),
		}
	}

	return tasks
}

func (d *Dealer) getPort(config *platformconfig.WebServer) (int, error) {
	_, portString, err := net.SplitHostPort(config.ListenAddress)
	if err != nil {
		return 0, err
	}

	port, err := strconv.Atoi(portString)
	if err != nil {
		return 0, err
	}

	return port, nil
}

// Post handles POST request
func (d *Dealer) Post(w http.ResponseWriter, r *http.Request) {
	encoder := json.NewEncoder(w)

	dealerRequest := Message{}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&dealerRequest); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		encoder.Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}

	processorConfigCopy := util.CopyConfiguration(d.processor.GetConfiguration())

	for jobID, job := range dealerRequest.Jobs {
		triggerConfig, found := processorConfigCopy.Spec.Triggers[jobID]
		if !found {
			w.WriteHeader(http.StatusBadRequest)
			encoder.Encode(map[string]string{
				"error": fmt.Sprintf("unknown trigger - %s", jobID),
			})
			return
		}

		triggerConfig.Partitions = make([]functionconfig.Partition, 0, len(job.Tasks))
		for _, task := range job.Tasks {
			checkpoint := fmt.Sprintf("%d", task.State)
			partition := functionconfig.Partition{
				ID:         fmt.Sprintf("%d", task.ID),
				Checkpoint: &checkpoint,
			}
			triggerConfig.Partitions = append(triggerConfig.Partitions, partition)
		}
		processorConfigCopy.Spec.Triggers[jobID] = triggerConfig
	}

	if err := d.processor.SetConfiguration(processorConfigCopy); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		encoder.Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}

	reply := d.createReply()
	updateConfiguration := d.processor.GetLastUpdate().GetConfiguration()
	for triggerID, trigger := range updateConfiguration.Spec.Triggers {
		job, ok := reply.Jobs[triggerID]
		if !ok {
			continue
		}

		for _, partition := range trigger.Partitions {
			id, err := strconv.Atoi(partition.ID)
			if err != nil {
				d.logger.WarnWith("partition without numeric ID", "id", partition.ID)
				continue
			}

			if d.inTasks(id, job.Tasks) {
				continue
			}

			task := Task{
				ID:         id,
				State:      TaskStateDeleted,
				Checkpoint: d.checkpointToStr(partition.Checkpoint),
			}
			job.Tasks = append(job.Tasks, task)
		}
	}
	encoder.Encode(reply)
}

func (d *Dealer) inTasks(id int, tasks []Task) bool {
	for _, task := range tasks {
		if task.ID == id {
			return true
		}
	}

	return false
}

func (d *Dealer) checkpointToStr(checkpoint functionconfig.Checkpoint) string {
	if checkpoint == nil {
		return emptyCheckpoint
	}

	return *checkpoint
}
