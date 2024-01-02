package utils

import (
<<<<<<< HEAD
=======
	"fmt"
>>>>>>> b56877031 (feat(pkg-nexus): models, scheduler, utils)
	"github.com/go-ping/ping"
	"github.com/nuclio/nuclio/pkg/nexus/common/models"
)

func GetEnvironmentHost() (host string) {
<<<<<<< HEAD
	_, err := ping.NewPinger("host.docker.internal")
=======
	s, err := ping.NewPinger("host.docker.internal")
	fmt.Println(s)
>>>>>>> b56877031 (feat(pkg-nexus): models, scheduler, utils)
	if err != nil {
		host = models.DEFAULT_HOST
	} else {
		host = models.DARWIN_HOST
	}
	return
}
