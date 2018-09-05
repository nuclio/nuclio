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

package nuclio

import (
	"errors"
	"strconv"
	"time"
)

// ErrUnsupported is returned when an unsupported interface on the event is called
var ErrUnsupported = errors.New("Event does not support this interface")

// ErrTypeConversion is returned when a type conversion for headers / fields fails
var ErrTypeConversion = errors.New("Cannot convert to this type")

// TriggerInfoProvider provides information about the trigger in which this event originated
type TriggerInfoProvider interface {

	// get the class of source (sync, async, etc)
	GetClass() string

	// get specific kind of source (http, rabbit mq, etc)
	GetKind() string
}

// Event allows access to the concrete event
type Event interface {

	// GetID returns the ID of the event
	GetID() ID

	// SetID sets the ID of the event
	SetID(ID)

	// SetTriggerInfoProvider sets the information about the trigger who triggered this event
	SetTriggerInfoProvider(TriggerInfoProvider)

	// GetTriggerInfo retruns a trigger info provider
	GetTriggerInfo() TriggerInfoProvider

	// GetContentType returns the content type of the body
	GetContentType() string

	// GetBody returns the body of the event
	GetBody() []byte

	// GetBodyObject returns the body of the event as an object
	GetBodyObject() interface{}

	// GetHeader returns the header by name as an interface{}
	GetHeader(string) interface{}

	// GetHeaderByteSlice returns the header by name as a byte slice
	GetHeaderByteSlice(string) []byte

	// GetHeaderString returns the header by name as a string
	GetHeaderString(string) string

	// GetHeaderInt returns the field by name as an integer
	GetHeaderInt(string) (int, error)

	// GetHeaders loads all headers into a map of string / interface{}
	GetHeaders() map[string]interface{}

	// GetField returns the field by name as an interface{}
	GetField(string) interface{}

	// GetFieldByteSlice returns the field by name as a byte slice
	GetFieldByteSlice(string) []byte

	// GetFieldString returns the field by name as a string
	GetFieldString(string) string

	// GetFieldInt returns the field by name as an integer
	GetFieldInt(string) (int, error)

	// GetFields loads all fields into a map of string / interface{}
	GetFields() map[string]interface{}

	// GetTimestamp returns when the event originated
	GetTimestamp() time.Time

	// GetPath returns the path of the event
	GetPath() string

	// GetURL returns the URL of the event
	GetURL() string

	// GetPath returns the method of the event, if applicable
	GetMethod() string

	// GetShardID returns the ID of the shard from which this event arrived, if applicable
	GetShardID() int

	// GetTotalNumShards returns the total number of shards, if applicable
	GetTotalNumShards() int

	// GetType returns the type of event
	GetType() string

	// GetTypeVersion returns the version of the type
	GetTypeVersion() string

	// GetVersion returns the version of the event
	GetVersion() string
}

// AbstractEvent provides a base implemention of an event
type AbstractEvent struct {
	triggerInfoProvider TriggerInfoProvider
	id                  ID
	emptyByteArray      []byte
	emptyHeaders        map[string]interface{}
	emptyTime           time.Time
}

// SetTriggerInfoProvider sets the information about the trigger who triggered this event
func (ae *AbstractEvent) SetTriggerInfoProvider(triggerInfoProvider TriggerInfoProvider) {
	ae.triggerInfoProvider = triggerInfoProvider
}

// GetTriggerInfo retruns a trigger info provider
func (ae *AbstractEvent) GetTriggerInfo() TriggerInfoProvider {
	return ae.triggerInfoProvider
}

// GetID returns the ID of the event
func (ae *AbstractEvent) GetID() ID {
	return ae.id
}

// SetID sets the ID of the event
func (ae *AbstractEvent) SetID(id ID) {
	ae.id = id
}

// GetContentType returns the content type of the body
func (ae *AbstractEvent) GetContentType() string {
	return ""
}

// GetBody returns the body of the event
func (ae *AbstractEvent) GetBody() []byte {
	return ae.emptyByteArray
}

// GetBodyObject returns the body of the event as an object
func (ae *AbstractEvent) GetBodyObject() interface{} {
	return ae.GetBody()
}

// GetHeader returns the header by name as an interface{}
func (ae *AbstractEvent) GetHeader(key string) interface{} {
	return nil
}

// GetHeaderByteSlice returns the header by name as a byte slice
func (ae *AbstractEvent) GetHeaderByteSlice(key string) []byte {
	return ae.emptyByteArray
}

// GetHeaderString returns the header by name as a string
func (ae *AbstractEvent) GetHeaderString(key string) string {
	return string(ae.GetHeaderByteSlice(key))
}

// GetHeaderInt returns the field by name as an integer
func (ae *AbstractEvent) GetHeaderInt(key string) (int, error) {

	// try to get header as an interface
	headerAsInterface := ae.GetHeader(key)

	// if the header value is not an integer
	switch typedHeader := headerAsInterface.(type) {
	case int:
		return typedHeader, nil
	case string:
		return strconv.Atoi(typedHeader)
	case []byte:
		return strconv.Atoi(string(typedHeader))

	default:
		return 0, ErrTypeConversion
	}
}

// GetHeaders loads all headers into a map of string / interface{}
func (ae *AbstractEvent) GetHeaders() map[string]interface{} {
	return ae.emptyHeaders
}

// GetTimestamp returns when the event originated
func (ae *AbstractEvent) GetTimestamp() time.Time {
	return ae.emptyTime
}

// GetPath returns the path of the event
func (ae *AbstractEvent) GetPath() string {
	return ""
}

// GetURL returns the URL of the event
func (ae *AbstractEvent) GetURL() string {
	return ""
}

// GetPath returns the method of the event, if applicable
func (ae *AbstractEvent) GetMethod() string {
	return ""
}

// GetField returns the field by name as an interface{}
func (ae *AbstractEvent) GetField(key string) interface{} {
	return nil
}

// GetFieldByteSlice returns the field by name as a byte slice
func (ae *AbstractEvent) GetFieldByteSlice(key string) []byte {
	return nil
}

// GetFieldString returns the field by name as a string
func (ae *AbstractEvent) GetFieldString(key string) string {
	return ""
}

// GetFieldInt returns the field by name as an integer
func (ae *AbstractEvent) GetFieldInt(key string) (int, error) {
	return 0, ErrUnsupported
}

// GetFields loads all fields into a map of string / interface{}
func (ae *AbstractEvent) GetFields() map[string]interface{} {
	return nil
}

// GetShardID returns the ID of the shard from which this event arrived, if applicable
func (ae *AbstractEvent) GetShardID() int {
	return -1
}

// GetTotalNumShards returns the total number of shards, if applicable
func (ae *AbstractEvent) GetTotalNumShards() int {
	return 0
}

// GetType returns the type of event
func (ae *AbstractEvent) GetType() string {
	return ""
}

// GetTypeVersion returns the version of the type
func (ae *AbstractEvent) GetTypeVersion() string {
	return ""
}

// GetVersion returns the version of the event
func (ae *AbstractEvent) GetVersion() string {
	return ""
}
