package iguazio
//
//import (
//	"github.com/nuclio/logger"
//	"github.com/nuclio/nuclio/pkg/platformconfig"
//	"time"
//)
//
//type Synchronizer struct {
//	logger                logger.Logger
//	platformConfiguration *platformconfig.Config
//}
//
//func NewSynchronizer(parentLogger logger.Logger, platformConfiguration *platformconfig.Config) (*Synchronizer, error) {
//	newSynchronizer := Synchronizer{
//		logger:                parentLogger.GetChild("leader-synchronizer-iguazio"),
//		platformConfiguration: platformConfiguration,
//	}
//
//	return &newSynchronizer, nil
//}
//
//func (c *Synchronizer) Start() {
//	c.logger.DebugWith("Started synchronization loop with leader", "intervalInSeconds", c.intervalInSeconds)
//
//	go c.synchronizeLoop()
//}
//
//func (c *Synchronizer) synchronizeLoop() error {
//	ticker := time.NewTicker(c.platformConfiguration.ProjectsLeader.SynchronizationInterval * time.Second)
//
//	for {
//		select {
//		case _ := <-ticker.C:
//			c.synchronizeProjectsAccordingToLeader()
//		}
//	}
//}
//
//func (c *Synchronizer) synchronizeLoop() error {
//
//}
