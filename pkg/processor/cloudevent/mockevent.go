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
	"github.com/stretchr/testify/mock"
)

type mockEvent struct { // nolint: deadcode
	mock.Mock
	triggerInfoProvider nuclio.TriggerInfoProvider
}

// GetID returns the ID of the event
func (me *mockEvent) GetID() nuclio.ID {
	args := me.Called()
	return args.Get(0).(nuclio.ID)
}

// SetID sets the ID of the event
func (me *mockEvent) SetID(id nuclio.ID) {
	me.Called()
}

// SetTriggerInfoProvider sets the information about the trigger who triggered this event
func (me *mockEvent) SetTriggerInfoProvider(triggerInfoProvider nuclio.TriggerInfoProvider) {
	me.Called(triggerInfoProvider)
	me.triggerInfoProvider = triggerInfoProvider
}

// GetTriggerInfo retruns a trigger info provider
func (me *mockEvent) GetTriggerInfo() nuclio.TriggerInfoProvider {
	return me.triggerInfoProvider
}

// GetContentType returns the content type of the body
func (me *mockEvent) GetContentType() string {
	args := me.Called()
	return args.String(0)
}

// GetBody returns the body of the event
func (me *mockEvent) GetBody() []byte {
	args := me.Called()
	return args.Get(0).([]byte)
}

// GetBodyObject returns the body of the event
func (me *mockEvent) GetBodyObject() interface{} {
	args := me.Called()
	return args.Get(0)
}

// GetHeader returns the header by name as an interface{}
func (me *mockEvent) GetHeader(key string) interface{} {
	args := me.Called(key)
	return args.String(0)
}

// GetHeaderByteSlice returns the header by name as a byte slice
func (me *mockEvent) GetHeaderByteSlice(key string) []byte {
	args := me.Called(key)
	return args.Get(0).([]byte)
}

// GetHeaderString returns the header by name as a string
func (me *mockEvent) GetHeaderString(key string) string {
	args := me.Called(key)
	return args.String(0)
}

// GetHeaderInt returns the field by name as an integer
func (me *mockEvent) GetHeaderInt(key string) (int, error) {
	args := me.Called(key)
	return args.Int(0), args.Error(1)
}

// GetHeaders loads all headers into a map of string / interface{}
func (me *mockEvent) GetHeaders() map[string]interface{} {
	args := me.Called()
	return args.Get(0).(map[string]interface{})
}

// GetField returns the field by name as an interface{}
func (me *mockEvent) GetField(key string) interface{} {
	args := me.Called(key)
	return args.Get(0).(interface{})
}

// GetFieldByteSlice returns the field by name as a byte slice
func (me *mockEvent) GetFieldByteSlice(key string) []byte {
	args := me.Called(key)
	return args.Get(0).([]byte)
}

// GetFieldString returns the field by name as a string
func (me *mockEvent) GetFieldString(key string) string {
	args := me.Called(key)
	return args.String(0)
}

// GetFieldInt returns the field by name as an integer
func (me *mockEvent) GetFieldInt(key string) (int, error) {
	args := me.Called(key)
	return args.Int(0), args.Error(1)
}

// GetFields loads all fields into a map of string / interface{}
func (me *mockEvent) GetFields() map[string]interface{} {
	args := me.Called()
	return args.Get(0).(map[string]interface{})
}

// GetTimestamp returns when the event originated
func (me *mockEvent) GetTimestamp() time.Time {
	args := me.Called()
	return args.Get(0).(time.Time)
}

// GetPath returns the path of the event
func (me *mockEvent) GetPath() string {
	args := me.Called()
	return args.String(0)
}

// GetURL returns the URL of the event
func (me *mockEvent) GetURL() string {
	args := me.Called()
	return args.String(0)
}

// GetMethod returns the method of the event, if applicable
func (me *mockEvent) GetMethod() string {
	args := me.Called()
	return args.String(0)
}

// GetShardID returns the ID of the shard from which this event arrived, if applicable
func (me *mockEvent) GetShardID() int {
	args := me.Called()
	return args.Int(0)
}

// GetTotalNumShards returns the total number of shards, if applicable
func (me *mockEvent) GetTotalNumShards() int {
	args := me.Called()
	return args.Int(0)
}

// GetType returns the type of event
func (me *mockEvent) GetType() string {
	args := me.Called()
	return args.String(0)
}

// GetTypeVersion returns the version of the type
func (me *mockEvent) GetTypeVersion() string {
	args := me.Called()
	return args.String(0)
}

// GetVersion returns the version of the event
func (me *mockEvent) GetVersion() string {
	args := me.Called()
	return args.String(0)
}
