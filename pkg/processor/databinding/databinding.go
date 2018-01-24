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

package databinding

import "github.com/nuclio/nuclio-sdk"

type DataBinding interface {

	// Start will start the data binding, connecting to the remote resource
	Start() error

	// Stop will stop the data binding, cleaning up resources and tearing down connections
	Stop() error

	// GetContextObject will return the object that is injected into the context
	GetContextObject() (interface{}, error)
}

type AbstractDataBinding struct {
	Logger nuclio.Logger
}

// Start will start the data binding, connecting to the remote resource
func (adb *AbstractDataBinding) Start() error {
	return nil
}

// Stop will stop the data binding, cleaning up resources and tearing down connections
func (adb *AbstractDataBinding) Stop() error {
	return nil
}

// GetContextObject will return the object that is injected into the context
func (adb *AbstractDataBinding) GetContextObject() (interface{}, error) {
	return nil, nil
}
