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
	"fmt"
	"time"
)

var (
	emptyCheckpoint = ""
)

// TODO: Add String method to all states

// TaskState is task state
type TaskState int8

// Possible task states
const (
	TaskStateUnassigned TaskState = 0
	TaskStateRunning    TaskState = 1
	TaskStateStopping   TaskState = 2
	TaskStateDeleted    TaskState = 3
	TaskStateAlloc      TaskState = 4
	TaskStateCompleted  TaskState = 5
)

func (ts TaskState) String() string {
	switch ts {
	case TaskStateUnassigned:
		return "TaskStateUnassigned"
	case TaskStateRunning:
		return "TaskStateRunning"
	case TaskStateStopping:
		return "TaskStateStopping"
	case TaskStateDeleted:
		return "TaskStateDeleted"
	case TaskStateAlloc:
		return "TaskStateAlloc"
	case TaskStateCompleted:
		return "TaskStateCompleted"
	}

	return fmt.Sprintf("UNKNOWN: %d", ts)
}

// ProcessState is processor state
type ProcessState int8

// Possible process states
const (
	ProcessStateUnknown  ProcessState = 0
	ProcessStateReady    ProcessState = 1
	ProcessStateNotReady ProcessState = 2
	ProcessStateFailed   ProcessState = 3
	ProcessStateDelete   ProcessState = 4
)

func (ps ProcessState) String() string {
	switch ps {
	case ProcessStateUnknown:
		return "ProcessStateUnknown"
	case ProcessStateReady:
		return "ProcessStateReady"
	case ProcessStateNotReady:
		return "ProcessStateNotReady"
	case ProcessStateFailed:
		return "ProcessStateFailed"
	case ProcessStateDelete:
		return "ProcessStateDelete"
	}

	return fmt.Sprintf("UNKNOWN: %d", ps)
}

// Task is a dealer task (trigger partition)
type Task struct {
	ID         int       `json:"id"`
	State      TaskState `json:"state"`
	Checkpoint string    `json:"checkpoint"`
}

// JobState is job state
type JobState int8

// Possible job states
const (
	JobStateUnknown    JobState = 0
	JobStateRunning    JobState = 1 // distributed to processes
	JobStateStopping   JobState = 2 // asking the processes to stop/free job task
	JobStateSuspended  JobState = 3 // user requested to suspend the job
	JobStateWaitForDep JobState = 4 // Job is waiting for the deployment to start
	JobStateScheduled  JobState = 5 // Job is scheduled for deployment
	JobStateCompleted  JobState = 6 // Job processing completed
)

func (js JobState) String() string {
	switch js {
	case JobStateUnknown:
		return "JobStateUnknown"
	case JobStateRunning:
		return "JobStateRunning"
	case JobStateStopping:
		return "JobStateStopping"
	case JobStateSuspended:
		return "JobStateSuspended"
	case JobStateWaitForDep:
		return "JobStateWaitForDep"
	case JobStateScheduled:
		return "JobStateScheduled"
	case JobStateCompleted:
		return "JobStateCompleted"
	}

	return fmt.Sprintf("UNKNOWN: %d", js)
}

// Job is a partition/shard
type Job struct {
	Tasks      []Task `json:"tasks"`
	TotalTasks int    `json:"totalTasks"`
	Disable    bool   `json:"disable"`
}

func newJob(tasks []Task, totalTasks int, disable bool) *Job {
	return &Job{
		Tasks:      tasks,
		TotalTasks: totalTasks,
		Disable:    disable,
	}
}

// Message is dealer request/response
type Message struct {
	Name        string    `json:"name"`
	Namespace   string    `json:"namespace"`
	Function    string    `json:"function"`
	Version     string    `json:"version,omitempty"`
	Alias       string    `json:"alias,omitempty"`
	IP          string    `json:"ip"`
	Port        int       `json:"port"`
	State       int       `json:"state"`
	TotalEvents uint64    `json:"totalEvents"`
	Timestamp   time.Time `json:"timestamp"`
	DealerURL   string    `json:"dealerURL"`

	Jobs map[string]*Job `json:"jobs"`
}
