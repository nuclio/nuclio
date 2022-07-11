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

package rpc

import (
	"github.com/nuclio/nuclio-sdk-go"
)

type EventEncoder interface {
	Encode(event nuclio.Event) error
}

type ControlEncoder interface {
	Encode(string) error
}

func eventAsMap(event nuclio.Event) map[string]interface{} {
	triggerInfo := event.GetTriggerInfo()
	eventToEncode := map[string]interface{}{
		"content_type": event.GetContentType(),
		"content-type": event.GetContentType(),
		"trigger": map[string]string{
			"kind": triggerInfo.GetKind(),
			"name": triggerInfo.GetName(),
		},
		"fields":       event.GetFields(),
		"headers":      event.GetHeaders(),
		"id":           event.GetID(),
		"method":       event.GetMethod(),
		"path":         event.GetPath(),
		"size":         len(event.GetBody()),
		"timestamp":    event.GetTimestamp().UTC().Unix(),
		"url":          event.GetURL(),
		"shard_id":     event.GetShardID(),
		"num_shards":   event.GetTotalNumShards(),
		"type":         event.GetType(),
		"type_version": event.GetTypeVersion(),
		"version":      event.GetVersion(),
	}
	return eventToEncode
}
