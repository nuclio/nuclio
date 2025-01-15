/*
Copyright 2023 The Nuclio Authors.

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

package status

import (
	"fmt"
	"sync"
)

// Provider is an interface for entities that have a reportable status
type Provider interface {

	// GetStatus Returns the entity's status
	GetStatus() Status
}

// Status is runtime status
type Status int

// Status codes
const (
	Initializing Status = iota
	Ready
	Error
	Stopped
)

func (s Status) String() string {
	switch s {
	case Initializing:
		return "initializing"
	case Ready:
		return "ready"
	case Error:
		return "error"
	case Stopped:
		return "stopped"
	}

	return fmt.Sprintf("Unknown status - %d", s)
}

func (s Status) OneOf(statuses ...Status) bool {
	for _, status := range statuses {
		if s == status {
			return true
		}
	}
	return false
}

type SafeStatus struct {
	status      Status
	statusMutex sync.RWMutex
}

func NewSafeStatus(status Status) *SafeStatus {
	return &SafeStatus{status: status, statusMutex: sync.RWMutex{}}
}

func (as *SafeStatus) SetStatus(status Status) {
	as.statusMutex.Lock()
	defer as.statusMutex.Unlock()

	as.status = status
}

func (as *SafeStatus) GetStatus() Status {
	as.statusMutex.RLock()
	defer as.statusMutex.RUnlock()

	return as.status
}

func (as *SafeStatus) String() string {
	return as.GetStatus().String()
}
