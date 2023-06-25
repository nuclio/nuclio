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

package main

import (
	"flag"
	"os"

	"github.com/nuclio/nuclio/cmd/controller/app"
	"github.com/nuclio/nuclio/pkg/common"

	"github.com/nuclio/errors"
)

func main() {
	var defaultResyncIntervalStr string

	if valueFromEnv := os.Getenv("NUCLIO_CONTROLLER_FUNCTION_OPERATOR_RESYNC_INTERVAL"); valueFromEnv != "" {
		defaultResyncIntervalStr = valueFromEnv
	} else {
		defaultResyncIntervalStr = common.GetEnvOrDefaultString("NUCLIO_CONTROLLER_RESYNC_INTERVAL", "10m")
	}

	kubeconfigPath := flag.String("kubeconfig-path", os.Getenv("KUBECONFIG"), "Path of kubeconfig file")
	namespace := flag.String("namespace", "", "Namespace to listen on, or * for all")
	imagePullSecrets := flag.String("image-pull-secrets", os.Getenv("NUCLIO_CONTROLLER_IMAGE_PULL_SECRETS"), "Optional secret name to use for pull")
	platformConfigurationPath := flag.String("platform-config", "/etc/nuclio/config/platform/platform.yaml", "Path of platform configuration file")
	platformConfigurationName := flag.String("platform-config-name", common.GetEnvOrDefaultString("NUCLIO_CONTROLLER_PLATFORM_CONFIGURATION_NAME", "nuclio-platform-config"), "Platform configuration resource name")
	functionOperatorNumWorkersStr := flag.String("function-operator-num-workers", common.GetEnvOrDefaultString("NUCLIO_CONTROLLER_FUNCTION_OPERATOR_NUM_WORKERS", "4"), "Set number of workers for the function operator (optional)")

	resyncIntervalStr := flag.String("resync-interval", defaultResyncIntervalStr, "Set resync interval for the function operator (optional)")

	// Deprecated: resync interval is commonly used by functions and api gateways
	deprecatedResyncIntervalStr := flag.String("function-operator-resync-interval", "", "Deprecated. Use --resync-interval instread")
	if deprecatedResyncIntervalStr != nil && *deprecatedResyncIntervalStr != "" {

		// ignore value, write deprecation note to stderr
		os.Stderr.WriteString("--function-operator-resync-interval has been deprecated in favor of --resync-interval.") // nolint: errcheck
	}

	functionMonitorIntervalStr := flag.String("function-monitor-interval", common.GetEnvOrDefaultString("NUCLIO_CONTROLLER_FUNCTION_MONITOR_INTERVAL", "3m"), "Set function monitor interval (optional)")
	cronJobStaleResourcesCleanupIntervalStr := flag.String("cron-job-stale-resources-cleanup-interval", common.GetEnvOrDefaultString("NUCLIO_CONTROLLER_CRON_JOB_STALE_RESOURCES_CLEANUP_INTERVAL", "1m"), "Set interval for the cleanup of stale cron job resources (optional)")
	evictedPodsCleanupIntervalStr := flag.String("evicted-pods-cleanup-interval", common.GetEnvOrDefaultString("NUCLIO_CONTROLLER_EVICTED_PODS_CLEANUP_INTERVAL", "30m"), "Set interval for the cleanup of evicted function pods (optional)")
	functionEventOperatorNumWorkersStr := flag.String("function-event-operator-num-workers", common.GetEnvOrDefaultString("NUCLIO_CONTROLLER_FUNCTION_EVENT_OPERATOR_NUM_WORKERS", "2"), "Set number of workers for the function event operator (optional)")
	projectOperatorNumWorkersStr := flag.String("project-operator-num-workers", common.GetEnvOrDefaultString("NUCLIO_CONTROLLER_PROJECT_OPERATOR_NUM_WORKERS", "2"), "Set number of workers for the project operator (optional)")
	apiGatewayOperatorNumWorkersStr := flag.String("api-gateway-operator-num-workers", common.GetEnvOrDefaultString("NUCLIO_CONTROLLER_API_GATEWAY_OPERATOR_NUM_WORKERS", "2"), "Set number of workers for the api gateway operator (optional)")

	flag.Parse()

	// get the namespace from args -> env -> default to self
	resolvedNamespace := common.ResolveNamespace(*namespace, "NUCLIO_CONTROLLER_NAMESPACE")

	if err := app.Run(*kubeconfigPath,
		resolvedNamespace,
		*imagePullSecrets,
		*platformConfigurationPath,
		*platformConfigurationName,
		*functionOperatorNumWorkersStr,
		*resyncIntervalStr,
		*functionMonitorIntervalStr,
		*cronJobStaleResourcesCleanupIntervalStr,
		*evictedPodsCleanupIntervalStr,
		*functionEventOperatorNumWorkersStr,
		*projectOperatorNumWorkersStr,
		*apiGatewayOperatorNumWorkersStr); err != nil {
		errors.PrintErrorStack(os.Stderr, err, 5)

		os.Exit(1)
	}
}
