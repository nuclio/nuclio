package interfaces

import (
	"github.com/konsumgandalf/profaastinate/nexus/common/models"
	common "github.com/konsumgandalf/profaastinate/nexus/common/models/configs"
)

// *nexus.Nexus
type INexusScheduler interface {
	Start(any, common.BaseNexusSchedulerConfig) *models.BaseNexusScheduler
}
