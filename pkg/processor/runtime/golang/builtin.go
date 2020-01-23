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

package golang

import (
	"github.com/nuclio/nuclio-sdk-go"
	"time"
)

var counters [64]int

// this is used for running a standalone processor during development
func builtInHandler(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
	if event.GetTriggerInfo().GetKind() == "kafka-cluster" {
		//context.Logger.InfoWith("Got event",
		//	"ShardID", event.GetShardID(),
		//	"Body", string(event.GetBody()))

		counters[event.GetShardID()]++
		time.Sleep(1 * time.Millisecond)
	} else {
		context.Logger.DebugWith("Counters", "counters", counters)
	}

	return "Built in handler called", nil
}
