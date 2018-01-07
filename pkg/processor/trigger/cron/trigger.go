package cron

import (
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/nuclio-sdk"
	cronlib "github.com/robfig/cron"
	"github.com/nuclio/nuclio/pkg/errors"
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
	interval      time.Duration
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

		newTrigger.interval, err = time.ParseDuration(interval.(string))
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to parse interval from cron trigger configuration", interval)
		}

		newTrigger.Logger.InfoWith("Creating new cron trigger with interval",
			"interval", newTrigger.interval)

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
		body:
		[]byte(configuration.Attributes["body"].(string)),
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
	var sleepDuration time.Duration
	lastRunTime := time.Now()

	for {
		select {
		case <-c.stop:
			c.Logger.Info("Cron trigger stop signal received")
			break
		default:
			c.handleTick()
		}

		if c.tickMethod == tickMethodInterval {
			sleepDuration = c.getNextSleepDurationInterval(lastRunTime)
		} else {
			sleepDuration = c.getNextSleepDurationSchedule(lastRunTime)
		}

		lastRunTime = time.Now()

		c.Logger.DebugWith("Event triggered. Waiting until next tick",
			"sleepDuration", sleepDuration)
		time.Sleep(sleepDuration)
	}
}

func (c *cron) getNextSleepDurationInterval(lastTick time.Time) time.Duration {
	missedTicks := 0
	nextTick := lastTick.Add(c.interval)

	for nextTick.Before(time.Now()) {
		nextTick = nextTick.Add(c.interval)
		missedTicks++
	}

	// Waiting a certain amount of time means that it will always "miss" the exact interval timeout by a short time
	// Ignoring the first "missed tick" to avoid double-triggering events
	if missedTicks > 1 {
		c.Logger.InfoWith("Missed runs. Running the latest interval",
			"missedRuns", missedTicks)
		return 0
	}

	return nextTick.Sub(time.Now())
}

func (c *cron) getNextSleepDurationSchedule(lastTick time.Time) time.Duration {
	missedTicks := 0
	nextTick := c.schedule.Next(lastTick)

	for nextTick.Before(time.Now()) {
		nextTick = c.schedule.Next(nextTick)
		missedTicks++
	}

	// Cron library will always wait until exactly the next event, meaning that it will always "miss" the expected event by a short time
	// Ignoring the first "missed tick" to avoid double-triggering events
	if missedTicks > 1 {
		c.Logger.InfoWith("Missed runs. Running the latest interval",
			"missedRuns", missedTicks)
		return 0
	}

	return time.Until(nextTick)
}

func (c *cron) handleTick() {
	c.baseEvent.headers["time"] = time.Now()

	c.AllocateWorkerAndSubmitEvent(
		&c.baseEvent,
		c.Logger,
		10*time.Second)
}
