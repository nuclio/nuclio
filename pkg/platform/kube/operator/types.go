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

import "k8s.io/apimachinery/pkg/runtime"

// ChangeHandler handles changes to object
type ChangeHandler interface {

	// CreateOrUpdate handles creation/update of an object
	CreateOrUpdate(runtime.Object) error

	// Delete handles delete of an object
	Delete(string, string) error
}
