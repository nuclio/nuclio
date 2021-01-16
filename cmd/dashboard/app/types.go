package app

import (
	"time"

	"github.com/nuclio/nuclio/pkg/common/healthcheck"
	"github.com/nuclio/nuclio/pkg/common/statusprovider"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/nuclio/logger"
)

type Dashboard struct {
	server            restful.Server
	healthCheckServer healthcheck.Server
	logger            logger.Logger

	status statusprovider.Status
}

func (d *Dashboard) GetStatus() statusprovider.Status {
	return d.status
}

func (d *Dashboard) SetStatus(status statusprovider.Status) {
	if d.status != status {
		d.logger.InfoWith("Updating server healthiness",
			"currentStatus", d.status,
			"desiredStatus", status)
	}
	d.status = status
}

func (d *Dashboard) MonitorDockerConnectivity(interval time.Duration,
	maxConsecutiveErrors int,
	dockerClient dockerclient.Client,
	stopChan <-chan struct{}) {

	consecutiveErrors := maxConsecutiveErrors
	dockerConnectivityTicker := time.NewTicker(interval)

	for {
		select {
		case <-stopChan:
			dockerConnectivityTicker.Stop()
			d.logger.DebugWith("Stopping docker connectivity monitor")
			return
		case <-dockerConnectivityTicker.C:
			if d.GetStatus().OneOf(statusprovider.Error) {

				// do not monitor while status is error
				// let the kubelet / dockerd / user restart the process
				continue
			}

			// get version quietly
			if _, err := dockerClient.GetVersion(true); err == nil {
				consecutiveErrors = maxConsecutiveErrors
				continue
			}
			consecutiveErrors--
			if consecutiveErrors == 0 {
				d.SetStatus(statusprovider.Error)
				d.logger.Error("Failed to resolve docker version, connection might be unhealthy.")
			}
		}
	}
}

type CreateDashboardServerOptions struct {
	logger                logger.Logger
	platformConfiguration *platformconfig.Config
	platformInstance      platform.Platform

	// arguments
	listenAddress                    string
	dockerKeyDir                     string
	defaultRegistryURL               string
	defaultRunRegistryURL            string
	platformType                     string
	noPullBaseImages                 bool
	defaultCredRefreshIntervalString string
	externalIPAddresses              string
	defaultNamespace                 string
	offline                          bool
	templatesGitRepository           string
	templatesGitRef                  string
	templatesArchiveAddress          string
	templatesGitUsername             string
	templatesGitPassword             string
	templatesGithubAccessToken       string
	defaultHTTPIngressHostTemplate   string
	imageNamePrefixTemplate          string
	platformAuthorizationMode        string
	dependantImageRegistryURL        string
}
