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

package kube

import (
	"fmt"

	"github.com/rs/xid"
)

type DeployOptions struct {
}

func IngressNameFromAPIGatewayName(apiGatewayName string, canary bool) string {
	resourceName := fmt.Sprintf("nuclio-agw-%s", apiGatewayName)
	if canary {
		resourceName += "-canary"
	}
	return resourceName
}

func BasicAuthNameFromAPIGatewayName(apiGatewayName string) string {
	return fmt.Sprintf("nuclio-agw-%s", apiGatewayName)
}

func DeploymentNameFromFunctionName(functionName string) string {
	return fmt.Sprintf("nuclio-%s", functionName)
}

func PodNameFromFunctionName(functionName string) string {
	return fmt.Sprintf("nuclio-%s", functionName)
}

func ConfigMapNameFromFunctionName(functionName string) string {
	return fmt.Sprintf("nuclio-%s", functionName)
}

func HPANameFromFunctionName(functionName string) string {
	return fmt.Sprintf("nuclio-%s", functionName)
}

func IngressNameFromFunctionName(functionName string) string {
	return fmt.Sprintf("nuclio-%s", functionName)
}

func ServiceNameFromFunctionName(functionName string) string {
	return fmt.Sprintf("nuclio-%s", functionName)
}

func CronJobName() string {
	return fmt.Sprintf("nuclio-cron-job-%s", xid.New().String())
}
