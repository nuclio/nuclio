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

package main

import (
	"encoding/json"
	"time"

	"github.com/nuclio/nuclio-sdk-go"
)

func EventReturner(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
	eventFields := struct {
		ID             nuclio.ID              `json:"id,omitempty"`
		TriggerClass   string                 `json:"triggerClass,omitempty"`
		TriggerKind    string                 `json:"eventType,omitempty"`
		ContentType    string                 `json:"contentType,omitempty"`
		Headers        map[string]interface{} `json:"headers,omitempty"`
		Timestamp      time.Time              `json:"timestamp,omitempty"`
		Path           string                 `json:"path,omitempty"`
		URL            string                 `json:"url,omitempty"`
		Method         string                 `json:"method,omitempty"`
		ShardID        int                    `json:"shardID,omitempty"`
		TotalNumShards int                    `json:"totalNumShards,omitempty"`
		Type           string                 `json:"type,omitempty"`
		TypeVersion    string                 `json:"typeVersion,omitempty"`
		Version        string                 `json:"version,omitempty"`
		Body           []byte                 `json:"body,omitempty"`
	} {
		ID: event.GetID(),
		TriggerClass: event.GetTriggerInfo().GetClass(),
		TriggerKind: event.GetTriggerInfo().GetKind(),
		ContentType: event.GetContentType(),
		Headers: event.GetHeaders(),
		Timestamp: event.GetTimestamp(),
		Path: event.GetPath(),
		URL: event.GetURL(),
		Method: event.GetMethod(),
		ShardID: event.GetShardID(),
		TotalNumShards: event.GetTotalNumShards(),
		Type: event.GetType(),
		TypeVersion: event.GetTypeVersion(),
		Version: event.GetVersion(),
		Body: event.GetBody(),
	}

	return json.Marshal(&eventFields)
}
