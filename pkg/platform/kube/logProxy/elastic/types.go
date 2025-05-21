/*
Copyright 2025 The Nuclio Authors.

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

package elastic

// versionInfo is a struct to hold the version information of the Elasticsearch or OpenSearch instance
type versionInfo struct {
	Version struct {
		// OpenSearch-specific
		Distribution string `json:"distribution"`

		// Both ES and OS have this
		Number string `json:"number"`
	} `json:"version"`
}
