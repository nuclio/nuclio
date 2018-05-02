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

package cloudevent

import (
	"time"

	"github.com/nuclio/nuclio-sdk-go"
)

type wrappedEvent struct {
	event nuclio.Event
}

// GetID returns the ID of the event
func (we *wrappedEvent) GetID() nuclio.ID {
	return we.event.GetID()
}

// SetID sets the ID of the event
func (we *wrappedEvent) SetID(id nuclio.ID) {
	we.event.SetID(id)
}

// SetTriggerInfoProvider sets the information about the trigger who triggered this event
func (we *wrappedEvent) SetTriggerInfoProvider(triggerInfoProvider nuclio.TriggerInfoProvider) {
	we.event.SetTriggerInfoProvider(triggerInfoProvider)
}

// GetTriggerInfo retruns a trigger info provider
func (we *wrappedEvent) GetTriggerInfo() nuclio.TriggerInfoProvider {
	return we.event.GetTriggerInfo()
}

// GetContentType returns the content type of the body
func (we *wrappedEvent) GetContentType() string {
	return we.event.GetContentType()
}

// GetBody returns the body of the event
func (we *wrappedEvent) GetBody() []byte {
	return we.event.GetBody()
}

// GetBodyObject returns the body of the event as an object
func (we *wrappedEvent) GetBodyObject() interface{} {
	return we.event.GetBodyObject()
}

// GetHeader returns the header by name as an interface{}
func (we *wrappedEvent) GetHeader(key string) interface{} {
	return we.event.GetHeader(key)
}

// GetHeaderByteSlice returns the header by name as a byte slice
func (we *wrappedEvent) GetHeaderByteSlice(key string) []byte {
	return we.event.GetHeaderByteSlice(key)
}

// GetHeaderString returns the header by name as a string
func (we *wrappedEvent) GetHeaderString(key string) string {
	return we.event.GetHeaderString(key)
}

// GetHeaderInt returns the field by name as an integer
func (we *wrappedEvent) GetHeaderInt(key string) (int, error) {
	return we.event.GetHeaderInt(key)
}

// GetHeaders loads all headers into a map of string / interface{}
func (we *wrappedEvent) GetHeaders() map[string]interface{} {
	return we.event.GetHeaders()
}

// GetField returns the field by name as an interface{}
func (we *wrappedEvent) GetField(key string) interface{} {
	return we.event.GetField(key)
}

// GetFieldByteSlice returns the field by name as a byte slice
func (we *wrappedEvent) GetFieldByteSlice(key string) []byte {
	return we.event.GetFieldByteSlice(key)
}

// GetFieldString returns the field by name as a string
func (we *wrappedEvent) GetFieldString(key string) string {
	return we.event.GetFieldString(key)
}

// GetFieldInt returns the field by name as an integer
func (we *wrappedEvent) GetFieldInt(key string) (int, error) {
	return we.event.GetFieldInt(key)
}

// GetFields loads all fields into a map of string / interface{}
func (we *wrappedEvent) GetFields() map[string]interface{} {
	return we.event.GetFields()
}

// GetTimestamp returns when the event originated
func (we *wrappedEvent) GetTimestamp() time.Time {
	return we.event.GetTimestamp()
}

// GetPath returns the path of the event
func (we *wrappedEvent) GetPath() string {
	return we.event.GetPath()
}

// GetURL returns the URL of the event
func (we *wrappedEvent) GetURL() string {
	return we.event.GetURL()
}

// GetPath returns the method of the event, if applicable
func (we *wrappedEvent) GetMethod() string {
	return we.event.GetMethod()
}

// GetShardID returns the ID of the shard from which this event arrived, if applicable
func (we *wrappedEvent) GetShardID() int {
	return we.event.GetShardID()
}

// GetTotalNumShards returns the total number of shards, if applicable
func (we *wrappedEvent) GetTotalNumShards() int {
	return we.event.GetTotalNumShards()
}

// GetType returns the type of event
func (we *wrappedEvent) GetType() string {
	return we.event.GetType()
}

// GetTypeVersion returns the version of the type
func (we *wrappedEvent) GetTypeVersion() string {
	return we.event.GetTypeVersion()
}

// GetVersion returns the version of the event
func (we *wrappedEvent) GetVersion() string {
	return we.event.GetVersion()
}
