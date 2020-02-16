/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cron

import (
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	cronlib "github.com/robfig/cron"
)

const (
	tickMethodSchedule = iota
	tickMethodInterval
)

type cron struct {
	trigger.AbstractTrigger
	configuration *Configuration
	tickMethod    int
	schedule      cronlib.Schedule
	stop          chan int
}

func newTrigger(logger logger.Logger,
	workerAllocator worker.Allocator,
	configuration *Configuration) (trigger.Trigger, error) {

	abstractTrigger, err := trigger.NewAbstractTrigger(logger,
		workerAllocator,
		&configuration.Configuration,
		"async",
		"cron",
		configuration.Name)
	if err != nil {
		return nil, errors.New("Failed to create abstract trigger")
	}

	newTrigger := cron{
		AbstractTrigger: abstractTrigger,
		configuration:   configuration,
		stop:            make(chan int),
	}

	if configuration.Interval != "" {
		newTrigger.tickMethod = tickMethodInterval

		var intervalLength time.Duration
		intervalLength, err = time.ParseDuration(configuration.Interval)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to parse interval from cron trigger configuration: %+v", configuration.Interval)
		}

		newTrigger.schedule = cronlib.ConstantDelaySchedule{
			Delay: intervalLength,
		}

		newTrigger.Logger.InfoWith("Creating new cron trigger with interval",
			"interval", intervalLength)

	} else if configuration.Schedule != "" {
		newTrigger.tickMethod = tickMethodSchedule

		// prevent the user from using * as Seconds
		splitSchedule := strings.Split(configuration.Schedule, " ")
		if splitSchedule[0] == "*" {
			splitSchedule[0] = "0"
		}
		normalizedSchedule := strings.Join(splitSchedule, " ")

		newTrigger.schedule, err = cronlib.Parse(normalizedSchedule)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to parse schedule from cron trigger configuration: %+v", configuration.Schedule)
		}

		newTrigger.Logger.InfoWith("Creating new cron trigger with schedule",
			"schedule", newTrigger.schedule)

	} else {
		return nil, errors.New("Cron trigger configuration must contain either interval or schedule")
	}

	return &newTrigger, nil
}

func (c *cron) Start(checkpoint functionconfig.Checkpoint) error {
	go c.handleEvents()
	return nil
}

func (c *cron) Stop(force bool) (functionconfig.Checkpoint, error) {
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
			c.waitAndSubmitNextEvent(lastRunTime, c.schedule, c.handleTick) // nolint: errcheck

			lastRunTime = time.Now()
		}

		if stop {
			break
		}
	}
}

func (c *cron) waitAndSubmitNextEvent(lastEventSubmitTime time.Time, schedule cronlib.Schedule, eventSubmitter func()) error {
	nextEventSubmitDelay := c.getNextEventSubmitDelay(schedule, lastEventSubmitTime)
	time.Sleep(nextEventSubmitDelay)

	eventSubmitter()

	return nil
}

func (c *cron) getNextEventSubmitDelay(schedule cronlib.Schedule, lastEventSubmitTime time.Time) time.Duration {
	var delay time.Duration

	// get when the next submit _should_ happen (might be in the past if we missed it)
	nextEventSubmitTime := c.calculateNextEventSubmittingTime(lastEventSubmitTime)

	// check how many events we missed
	missedTicks := c.getMissedTicks(schedule, nextEventSubmitTime)

	// if we missed some runs, return zero delay (aka, execute now)
	if missedTicks > 0 {
		c.Logger.InfoWith("Missed runs",
			"missedRuns", missedTicks)
		delay = 0
	} else {
		delay = time.Until(nextEventSubmitTime)
	}

	return delay
}

func (c *cron) getMissedTicks(schedule cronlib.Schedule, eventSubmitTime time.Time) int {
	var missedTicks int

	for eventSubmitTime.Before(time.Now()) {
		eventSubmitTime = c.calculateNextEventSubmittingTime(eventSubmitTime)
		missedTicks++
	}

	// last "missed" tick shouldn't count, as it wouldn't have happened yet (as we passed time.Now())
	// can't have missed less than 0 ticks
	if missedTicks > 0 {
		missedTicks--
	}

	return missedTicks
}

func (c *cron) calculateNextEventSubmittingTime(lastEventSubmitTime time.Time) time.Time {
	switch c.tickMethod {
	case tickMethodSchedule:
		return c.schedule.Next(lastEventSubmitTime)
	case tickMethodInterval:
		delay := c.schedule.(cronlib.ConstantDelaySchedule).Delay
		return lastEventSubmitTime.Add(delay)
	default:
		return time.Now()
	}
}

func (c *cron) handleTick() {
	c.AllocateWorkerAndSubmitEvent( // nolint: errcheck
		&c.configuration.Event,
		c.Logger,
		10*time.Second)
}
