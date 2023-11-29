package task

import (
	"time"
)

type Task struct {
	Name     string
	Deadline time.Time
}

func NewTask(name string, deadline time.Time) *Task {
	return &Task{name, deadline}
}
