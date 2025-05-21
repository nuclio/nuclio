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

// CreateLogProxy connects to the given endpoint and resolves whether it is an OpenSearch or Elasticsearch instance
// It returns a LogProxy implementation accordingly
func CreateLogProxy(logger logger.Logger, config *platformconfig.ElasticSearchConfig) (logProxy.LogProxy, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(config.URL)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to connect to the elastic endpoint")
	}

	defer resp.Body.Close()
	var versionInfoInstance *versionInfo

	if versionInfoInstance, err = getVersionFromSearchEngineWithRetries(config.URL,
		3,
		2*time.Second,
		15*time.Second); err != nil {
		return nil, errors.Wrap(err, "Failed to get version from search engine")
	}

	// Determine the backend based on the `distribution` field
	// only opensearch has this field
	switch versionInfoInstance.Version.Distribution {
	case "opensearch":
		logger.InfoWith("Creating log proxy client",
			"distribution", versionInfoInstance.Version.Distribution,
			"version", versionInfoInstance.Version.Number)

		return NewOpenSearchLogProxy(config)
	default:
		logger.InfoWith("Creating log proxy client",
			"distribution", "elasticsearch",
			"version", versionInfoInstance.Version.Number)

		return NewElasticSearchLogProxy(config)
	}
}

func getVersionFromSearchEngineWithRetries(
	url string,
	maxRetries int,
	retryInterval time.Duration,
	timeout time.Duration,
) (versionInfoInstance *versionInfo, err error) {

	client := &http.Client{Timeout: timeout}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		versionInfoInstance, err = getVersionFromSearchEngine(client, url)
		if err == nil {
			return
		}

		if attempt < maxRetries {
			time.Sleep(retryInterval)
		}
	}
	return
}

func getVersionFromSearchEngine(client *http.Client, url string) (*versionInfo, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to connect to Elastic endpoint")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("Received non-OK HTTP status: %s", resp.Status)
	}

	var version versionInfo
	if err := json.NewDecoder(resp.Body).Decode(&version); err != nil {
		return nil, errors.Wrap(err, "Failed to decode Elastic version response")
	}

	return &version, nil
}
