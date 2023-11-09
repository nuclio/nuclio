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

package app

import (
	"context"
	"sync/atomic"
	"time"

	commonhealthcheck "github.com/nuclio/nuclio/pkg/common/healthcheck"
	"github.com/nuclio/nuclio/pkg/common/status"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/nuclio/logger"
)

type Dashboard struct {
	server            restful.Server
	healthCheckServer commonhealthcheck.Server
	logger            logger.Logger

	status atomic.Value
}

func NewDashboard(logger logger.Logger) *Dashboard {
	d := &Dashboard{
		status: atomic.Value{},
		logger: logger,
	}
	d.status.Store(status.Initializing)
	return d
}

func (d *Dashboard) GetStatus() status.Status {
	return d.status.Load().(status.Status)
}

func (d *Dashboard) SetStatus(status status.Status) {
	if d.status.CompareAndSwap(d.GetStatus(), status) {
		d.logger.InfoWith("Updating server healthiness",
			"currentStatus", d.status,
			"desiredStatus", status)
	}
}

func (d *Dashboard) monitorDockerConnectivity(ctx context.Context,
	interval time.Duration,
	maxConsecutiveErrors int,
	dockerClient dockerclient.Client) {

	d.logger.DebugWith("Monitoring docker connectivity",
		"interval", interval,
		"maxConsecutiveErrors", maxConsecutiveErrors)

	consecutiveErrors := 0
	dockerConnectivityTicker := time.NewTicker(interval)
	for {
		select {
		case <-ctx.Done():
			dockerConnectivityTicker.Stop()
			d.logger.DebugWith("Stopping docker connectivity monitor")
			return
		case <-dockerConnectivityTicker.C:
			if d.GetStatus().OneOf(status.Error) {

				// do not monitor while status is error
				// let the kubelet / dockerd / user restart the process
				continue
			}

			// get version quietly, avoid execution spamming stdout
			if _, err := dockerClient.GetVersion(true); err == nil {

				// reset counter
				consecutiveErrors = 0
				continue
			}
			consecutiveErrors++
			if consecutiveErrors == maxConsecutiveErrors {
				d.SetStatus(status.Error)
				d.logger.Error("Failed to resolve docker version, connection might be unhealthy.")
			}
		}
	}
}
