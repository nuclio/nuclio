package utils

import (
<<<<<<< HEAD
<<<<<<< HEAD
=======
	"fmt"
>>>>>>> b56877031 (feat(pkg-nexus): models, scheduler, utils)
=======
>>>>>>> 51b03bcaa (refactor(pkg-nexus): logging)
	"github.com/go-ping/ping"
	"github.com/nuclio/nuclio/pkg/nexus/common/models"
)

func GetEnvironmentHost() (host string) {
<<<<<<< HEAD
<<<<<<< HEAD
	_, err := ping.NewPinger("host.docker.internal")
=======
	s, err := ping.NewPinger("host.docker.internal")
	fmt.Println(s)
>>>>>>> b56877031 (feat(pkg-nexus): models, scheduler, utils)
=======
	_, err := ping.NewPinger("host.docker.internal")
>>>>>>> 51b03bcaa (refactor(pkg-nexus): logging)
	if err != nil {
		host = models.DEFAULT_HOST
	} else {
		host = models.DARWIN_HOST
	}
	return
}
