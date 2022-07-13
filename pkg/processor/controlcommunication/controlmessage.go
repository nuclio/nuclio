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

package controlcommunication

import (
	"encoding/json"

	"github.com/nuclio/errors"
	"github.com/nuclio/nuclio-sdk-go"
)

type ControlMessageEvent struct {
	nuclio.AbstractEvent
	resolvedBody *ControlMessage
}

// GetID returns the ID of the event
func (cme *ControlMessageEvent) GetID() nuclio.ID {
	return nuclio.ID(cme.resolvedBody.Kind)
}

// GetBodyObject returns the control message body of the event
func (cme *ControlMessageEvent) GetBodyObject() (interface{}, error) {
	a := cme.GetBody()

	// lazy load
	if cme.resolvedBody != nil {
		return cme.resolvedBody, nil
	}
	message := &ControlMessage{}
	if err := json.Unmarshal(a, message); err != nil {
		return nil, errors.Wrap(err, "Failed unmarshalling event body")
	}
	cme.resolvedBody = message
	return message, nil
}
