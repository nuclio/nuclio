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

// Package util holds various processor utilities
package util

import (
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor"
)

// CopyConfiguration returns a "safe" copy of the processor configuration
func CopyConfiguration(config *processor.Configuration) *processor.Configuration {
	newConfig := *config

	triggersCopy := make(map[string]functionconfig.Trigger)
	for triggerID, trigger := range config.Spec.Triggers {
		triggersCopy[triggerID] = trigger
	}

	newConfig.Spec.Triggers = triggersCopy
	// TODO: Copy other by-ref fields

	return &newConfig
}
