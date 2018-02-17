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

package functioncr

import (
	"github.com/nuclio/nuclio/pkg/errors"
)

// returns true if function is ready
func WaitConditionReady(functioncrInstance *Function) (bool, error) {
	switch functioncrInstance.Status.State {
	case FunctionStateReady:
		return true, nil
	case FunctionStateError:
		return false, errors.Errorf("Function in error state (%s)", functioncrInstance.Status.Message)
	}

	return false, nil
}
