package cron

import (
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/nuclio-sdk"
	cronlib "github.com/robfig/cron"
)

const (
	tickMethodSchedule = iota
	tickMethodInterval
)

type cron struct {
	trigger.AbstractTrigger
	configuration *Configuration
	baseEvent     Event
	tickMethod    int
	schedule      cronlib.Schedule
	stop          chan int
}

func newTrigger(logger nuclio.Logger,
	workerAllocator worker.Allocator,
	configuration *Configuration) (trigger.Trigger, error) {

	newTrigger := cron{
		AbstractTrigger: trigger.AbstractTrigger{
			ID:              configuration.ID,
			Logger:          logger,
			WorkerAllocator: workerAllocator,
			Class:           "sync",
			Kind:            "cron",
		},
		configuration: configuration,
		stop:          make(chan int),
	}
	var err error

	if configuration.Interval != "" {
		newTrigger.tickMethod = tickMethodInterval

		var intervalLength time.Duration
		intervalLength, err = time.ParseDuration(configuration.Interval)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to parse interval from cron trigger configuration", configuration.Interval)
		}

		newTrigger.schedule = cronlib.ConstantDelaySchedule{
			Delay: intervalLength,
		}

		newTrigger.Logger.InfoWith("Creating new cron trigger with interval",
			"interval", intervalLength)

	} else if configuration.Schedule != "" {
		newTrigger.tickMethod = tickMethodSchedule

		newTrigger.schedule, err = cronlib.Parse(configuration.Schedule)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to parse schedule from cron trigger configuration", configuration.Schedule)
		}

		newTrigger.Logger.InfoWith("Creating new cron trigger with schedule",
			"schedule", newTrigger.schedule)

	} else {
		return nil, errors.New("Cron trigger configuration must contain either interval or schedule")
	}

	newTrigger.baseEvent = configuration.Event

	return &newTrigger, nil
}

func (c *cron) Start(checkpoint trigger.Checkpoint) error {
	go c.handleEvents()
	return nil
}

func (c *cron) Stop(force bool) (trigger.Checkpoint, error) {
	close(c.stop)

	return nil, nil
}

func (c *cron) GetConfig() map[string]interface{} {
	return common.StructureToMap(c.configuration)
}

func (c *cron) handleEvents() {
	lastRunTime := time.Now()
	stop := false

	for {
		select {
		case <-c.stop:
			c.Logger.Info("Cron trigger stop signal received")
			stop = true
		default:
			c.waitAndSubmitNextEvent(lastRunTime, c.schedule, c.handleTick)

			lastRunTime = time.Now()
		}

		if stop {
			break
		}
	}
}

func (c *cron) waitAndSubmitNextEvent(lastEventSubmitTime time.Time, schedule cronlib.Schedule, eventSubmitter func()) error {
	nextEventSubmitDelay := c.getNextEventSubmitDelay(schedule, lastEventSubmitTime)
	c.Logger.DebugWith("Waiting for next event",
		"delay", nextEventSubmitDelay)

	time.Sleep(nextEventSubmitDelay)

	c.Logger.Debug("Submitting event")
	eventSubmitter()

	return nil
}

func (c *cron) getNextEventSubmitDelay(schedule cronlib.Schedule, lastEventSubmitTime time.Time) time.Duration {

	// get when the next submit _should_ happen (might be in the past if we missed it)
	nextEventSubmitTime := schedule.Next(lastEventSubmitTime)

	// check if and how many events we missed and forward to the next event time that is in the future
	missedTicks := c.getMissedTicks(schedule, nextEventSubmitTime)
	for i := 0; i < missedTicks; i++ {
		nextEventSubmitTime = schedule.Next(nextEventSubmitTime)
	}

	if missedTicks > 0 {
		c.Logger.InfoWith("Missed runs. Running the latest interval",
			"missedRuns", missedTicks)
		return 0
	}

	return time.Until(nextEventSubmitTime)
}

func (c *cron) getMissedTicks(schedule cronlib.Schedule, nextEventSubmitTime time.Time) int {
	missedTicks := 0

	for nextEventSubmitTime.Before(time.Now()) {
		nextEventSubmitTime = schedule.Next(nextEventSubmitTime)
		missedTicks++
	}

	// Received next event submit time, so the last "missed" tick shouldn't count, as it wouldn't have happened yet
	// Can't have missed less than 0 ticks
	if missedTicks > 0 {
		return missedTicks - 1
	}

	return missedTicks
}

func (c *cron) handleTick() {
	c.AllocateWorkerAndSubmitEvent(
		&c.baseEvent,
		c.Logger,
		10*time.Second)
}
