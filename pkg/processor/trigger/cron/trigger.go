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

	if interval, ok := configuration.Attributes["interval"]; ok {
		newTrigger.tickMethod = tickMethodInterval

		intervalLength, err := time.ParseDuration(interval.(string))
		newTrigger.schedule = cronlib.ConstantDelaySchedule{
			Delay: intervalLength,
		}
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to parse interval from cron trigger configuration", interval)
		}

		newTrigger.Logger.InfoWith("Creating new cron trigger with interval",
			"interval", intervalLength)

	} else if schedule, ok := configuration.Attributes["schedule"]; ok {
		newTrigger.tickMethod = tickMethodSchedule

		newTrigger.schedule, err = cronlib.Parse(schedule.(string))
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to parse schedule from cron trigger configuration", schedule.(string))
		}

		newTrigger.Logger.InfoWith("Creating new cron trigger with schedule",
			"schedule", newTrigger.schedule)

	} else {
		return nil, errors.New("Cron trigger configuration must contain either interval or schedule")
	}

	newTrigger.baseEvent = Event{
		body:    []byte(configuration.Attributes["body"].(string)),
		headers: configuration.Attributes["headers"].(map[string]interface{}),
	}

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
	missedTicks := c.getMissedTicks(lastEventSubmitTime, schedule)
	for i := 0; i < missedTicks; i++ {
		nextEventSubmitTime = schedule.Next(nextEventSubmitTime)
	}

	// Waiting a certain amount of time means that it will always "miss" the exact interval timeout by a short time
	// Ignoring the first "missed tick" to avoid double-triggering events
	if missedTicks > 1 {
		c.Logger.InfoWith("Missed runs. Running the latest interval",
			"missedRuns", missedTicks)
		return 0
	}

	return time.Until(nextEventSubmitTime)
}

func (c *cron) getMissedTicks(lastEventSubmitTime time.Time, schedule cronlib.Schedule) int {
	missedTicks := 0

	nextEventSubmitTime := c.schedule.Next(lastEventSubmitTime)

	for nextEventSubmitTime.Before(time.Now()) {
		nextEventSubmitTime = c.schedule.Next(nextEventSubmitTime)
		missedTicks++
	}

	return missedTicks
}

func (c *cron) handleTick() {
	c.AllocateWorkerAndSubmitEvent(
		&c.baseEvent,
		c.Logger,
		10*time.Second)
}
