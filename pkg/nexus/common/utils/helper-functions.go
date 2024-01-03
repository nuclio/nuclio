package utils

import (
	"github.com/go-ping/ping"
	"github.com/nuclio/nuclio/pkg/nexus/common/models"
)

func GetEnvironmentHost() (host string) {
	_, err := ping.NewPinger("host.docker.internal")
	if err != nil {
		host = models.DEFAULT_HOST
	} else {
		host = models.DARWIN_HOST
	}
	return
}
