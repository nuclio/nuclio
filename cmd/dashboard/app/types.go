package app

import (
	"time"

	"github.com/nuclio/nuclio/pkg/common/healthcheck"
	"github.com/nuclio/nuclio/pkg/common/statusprovider"
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
	maxConsecutiveErrors int) {
	// TODO: uncomment
	//consecutiveErrors := maxConsecutiveErrors
	//dockerConnectivityTicker := time.NewTicker(interval)
	//for range dockerConnectivityTicker.C {
	//	if _, err := d.dockerClient.GetVersion(); err != nil {
	//		consecutiveErrors--
	//	} else {
	//		consecutiveErrors = maxConsecutiveErrors
	//		continue
	//	}
	//	if consecutiveErrors == 0 {
	//		d.SetStatus(statusprovider.Error)
	//		d.logger.Error("Failed to resolve docker version, connection might be unhealthy")
	//		break
	//	}
	//}
}

type CreateDashboardServerOptions struct {
	logger                logger.Logger
	platformConfiguration *platformconfig.Config

	ListenAddress                    string
	DockerKeyDir                     string
	DefaultRegistryURL               string
	DefaultRunRegistryURL            string
	PlatformType                     string
	NoPullBaseImages                 bool
	DefaultCredRefreshIntervalString string
	ExternalIPAddresses              string
	DefaultNamespace                 string
	Offline                          bool
	PlatformConfigurationPath        string
	TemplatesGitRepository           string
	TemplatesGitRef                  string
	TemplatesArchiveAddress          string
	TemplatesGitUsername             string
	TemplatesGitPassword             string
	TemplatesGithubAccessToken       string
	DefaultHTTPIngressHostTemplate   string
	ImageNamePrefixTemplate          string
	PlatformAuthorizationMode        string
	DependantImageRegistryURL        string
}
