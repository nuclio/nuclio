package utils

import (
	"fmt"
	"github.com/go-ping/ping"
	"github.com/nuclio/nuclio/pkg/nexus/common/models"
)

func GetEnvironmentHost() (host string) {
	s, err := ping.NewPinger("host.docker.internal")
	fmt.Println(s)
	if err != nil {
		host = models.DEFAULT_HOST
	} else {
		host = models.DARWIN_HOST
	}
	return
}
