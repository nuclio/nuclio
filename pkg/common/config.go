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
