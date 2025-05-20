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

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/nuclio/nuclio/pkg/platform/kube/logProxy"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type BaseElasticLikeLogProxy struct {
	index            string
	customQueryParam string
}

// versionInfo is a struct to hold the version information of the Elasticsearch or OpenSearch instance
type versionInfo struct {
	Version struct {
		// OpenSearch-specific
		Distribution string `json:"distribution"`

		// Both ES and OS have this
		Number string `json:"number"`
	} `json:"version"`
}

// ResolveLogProxy connects to the given endpoint and resolves whether it is an OpenSearch or Elasticsearch instance
// It returns a LogProxy implementation accordingly
func ResolveLogProxy(logger logger.Logger, config *platformconfig.ElasticSearchConfig) (logProxy.LogProxy, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(config.URL)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to connect to the elastic endpoint")
	}

	defer resp.Body.Close()

	var versionInfoInstance *versionInfo
	if err := json.NewDecoder(resp.Body).Decode(&versionInfoInstance); err != nil {
		return nil, errors.Wrap(err, "Failed to decode elastic version response")
	}

	// Determine the backend based on the `distribution` field
	// only opensearch has this field
	switch versionInfoInstance.Version.Distribution {
	case "opensearch":
		logger.InfoWith("Creating elastic-like log proxy client",
			"distribution", versionInfoInstance.Version.Distribution,
			"version", versionInfoInstance.Version.Number)

		return NewOpenSearchLogProxy(config)
	default:
		logger.InfoWith("Creating elastic-like log proxy client",
			"distribution", "elasticsearch",
			"version", versionInfoInstance.Version.Number)

		return NewElasticSearchLogProxy(config)
	}
}
