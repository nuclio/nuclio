/*
Copyright 2023 The Nuclio Authors.

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

package common

import "k8s.io/api/core/v1"

func EnvInSlice(env v1.EnvVar, slice []v1.EnvVar) bool {
	for _, envVar := range slice {
		if envVar.Name == env.Name {
			return true
		}
	}
	return false
}

func RemoveEnvFromSlice(env v1.EnvVar, slice []v1.EnvVar) []v1.EnvVar {
	for i, envVar := range slice {
		if envVar.Name == env.Name {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

// MergeEnvSlices merges two lists of environment variables, giving priority to variables from the primary list
func MergeEnvSlices(primaryEnv []v1.EnvVar, secondaryEnv []v1.EnvVar) []v1.EnvVar {
	envMap := make(map[string]v1.EnvVar)

	// add environment variables from the secondary list to the map
	for _, env := range secondaryEnv {
		envMap[env.Name] = env
	}

	// add environment variables from the primary list to the map with priority
	for _, env := range primaryEnv {
		envMap[env.Name] = env
	}

	// convert the map back to a slice of EnvVar
	mergedEnv := make([]v1.EnvVar, 0, len(envMap))
	for _, env := range envMap {
		mergedEnv = append(mergedEnv, env)
	}

	return mergedEnv
}
