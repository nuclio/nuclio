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

import "fmt"

// returns true if function was processed
func WaitConditionProcessed(functioncrInstance *Function) (bool, error) {

	// TODO: maybe possible that error existed before and our new post wasnt yet updated to status created ("")
	if functioncrInstance.Status.State != FunctionStateCreated {
		if functioncrInstance.Status.State == FunctionStateError {
			return true, fmt.Errorf("Function in error state (%s)", functioncrInstance.Status.Message)
		}

		return true, nil
	}

	return false, nil
}
