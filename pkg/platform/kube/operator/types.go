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

package operator

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
)

// Key to use when setting the worker id.
type ctxKeyWorkerID int

// WorkerIDKey is the key that holds the worker id.
const WorkerIDKey ctxKeyWorkerID = 0

// ChangeHandler handles changes to object
type ChangeHandler interface {

	// CreateOrUpdate handles creation/update of an object
	CreateOrUpdate(context.Context, runtime.Object) error

	// Delete handles delete of an object
	Delete(context.Context, string, string) error
}
