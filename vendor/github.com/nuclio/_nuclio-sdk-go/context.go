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

import "github.com/nuclio/logger"

// Context holds objects whose lifetime is that of the function instance
type Context struct {

	// Logger allows submitting information to logger sinks configured in the platform
	Logger logger.Logger

	// DataBinding holds a map of <data binding name> <data binding object>. For example, if the user
	// configured the function to bind to an Azure Event Hub, it will hold an instance of an Event Hub
	// client. The user can type cast this to the client type
	DataBinding map[string]DataBinding

	// Platform is set of platform-specific functions like "invoking other function"
	Platform *Platform

	// WorkerID holds the unique identifier of the worker currently handling the event. It can be used
	// to key into shared datasets to prevent locking
	WorkerID int

	// UserData is nil by default. This holds information set by the user should he need access to long
	// living data. The lifetime of this pointer is that of the _worker_ and workers can come and go.
	// Treat this like cache - always check if it's nil prior to access and re-populate if necessary
	UserData interface{}

	// FunctionName holds the name of the function currently running
	FunctionName string

	// FunctionVersion holds the version of the function currently running
	FunctionVersion int
}
