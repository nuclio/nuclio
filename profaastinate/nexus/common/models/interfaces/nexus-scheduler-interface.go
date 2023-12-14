package interfaces

import (
	"nexus/common/models"
	common "nexus/common/models/configs"
)

// *nexus.Nexus
type INexusScheduler interface {
	Start(any, common.BaseNexusSchedulerConfig) *models.BaseNexusScheduler
}
