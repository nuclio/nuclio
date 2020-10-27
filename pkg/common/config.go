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
