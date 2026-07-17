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

package processor

import (
	"github.com/nuclio/nuclio/pkg/common/headers"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/nuclio-sdk-go"
)

// Configuration is processor configuration
type Configuration struct {
	functionconfig.Config
	PlatformConfig *platformconfig.Config
}

type StreamNoAckError struct{}

func (s StreamNoAckError) Error() string {
	return "stream-no-ack"
}

// EventDiscardedDuringDrain reports whether the runtime discarded the event without
// handing it to the handler because the worker was draining (function restart / stream
// rebalance). Such events must not be acked, otherwise the stream trigger commits their
// offset and they are silently lost. The wrapper signals this via the StreamEventDiscarded
// response header; stream triggers convert it into a StreamNoAckError so the event is
// redelivered once the worker is back, preserving at-least-once delivery (NUC-855).
func EventDiscardedDuringDrain(response nuclio.ProcessingResult) bool {
	if response == nil {
		return false
	}

	discarded, ok := response.GetHeaders()[headers.StreamEventDiscarded].(bool)
	return ok && discarded
}
