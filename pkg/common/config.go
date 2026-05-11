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

// MergeEnvFromSlices merges two lists of EnvFromSource, giving priority to entries from the primary list.
// Deduplication is based on source type, name, and prefix.
func MergeEnvFromSlices(primaryEnvFrom []v1.EnvFromSource, secondaryEnvFrom []v1.EnvFromSource) []v1.EnvFromSource {
	existing := make(map[string]bool)
	for _, envSource := range primaryEnvFrom {
		existing[envFromSourceKey(envSource)] = true
	}
	merged := append([]v1.EnvFromSource{}, primaryEnvFrom...)
	for _, envSource := range secondaryEnvFrom {
		if !existing[envFromSourceKey(envSource)] {
			merged = append(merged, envSource)
		}
	}
	return merged
}

// envFromSourceKey returns a unique string key for an EnvFromSource based on its source type (secret or configmap),
// name, and optional prefix. Two entries with the same secret/configmap name but different prefixes are
// considered distinct, as they inject different env var names into the container.
func envFromSourceKey(envSource v1.EnvFromSource) string {
	prefix := envSource.Prefix
	if envSource.SecretRef != nil {
		return "secret:" + prefix + ":" + envSource.SecretRef.Name
	}
	if envSource.ConfigMapRef != nil {
		return "configmap:" + prefix + ":" + envSource.ConfigMapRef.Name
	}
	return ""
}

// MergeEnvSlices merges two lists of environment variables, giving priority to variables from the primary list
func MergeEnvSlices(primaryEnv []v1.EnvVar, secondaryEnv []v1.EnvVar) []v1.EnvVar {
	envMap := make(map[string]v1.EnvVar)

	// add environment variables from the secondary list to the map
	for _, env := range secondaryEnv {
		envMap[env.Name] = env
	}

	// add environment variables from the primary list to the map, overriding secondary list variables if the keys are the same
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
