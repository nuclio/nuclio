/*
Copyright 2023 The Nuclio Authors.

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

package natsjetstream

import (
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger"

	"github.com/mitchellh/mapstructure"
	nats "github.com/nats-io/nats.go"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type Configuration struct {
	trigger.Configuration
	Stream   string
	Consumer string

	// NATS connection configuration
	AllowReconnect  bool
	MaxReconnect    int
	ReconnectWait   string
	ReconnectJitter string
	Timeout         string
	DrainTimeout    string
	FlusherTimeout  string
	PingInterval    string
	MaxPingsOut     int

	// resolved fields
	reconnectWait   time.Duration
	reconnectJitter time.Duration
	timeout         time.Duration
	drainTimeout    time.Duration
	flusherTimeout  time.Duration
	pingInterval    time.Duration
}

func NewConfiguration(logger logger.Logger,
	id string,
	triggerConfiguration *functionconfig.Trigger,
	runtimeConfiguration *runtime.Configuration) (*Configuration, error) {
	newConfiguration := Configuration{}

	// create base
	baseConfiguration, err := trigger.NewConfiguration(id, triggerConfiguration, runtimeConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create trigger configuration")
	}
	newConfiguration.Configuration = *baseConfiguration

	// parse attributes
	if err := mapstructure.Decode(newConfiguration.Attributes, &newConfiguration); err != nil {
		return nil, errors.Wrap(err, "Failed to decode attributes")
	}

	err = newConfiguration.PopulateConfigurationFromAnnotations([]trigger.AnnotationConfigField{
		{Key: "nuclio.io/nats-jetstream-allow-reconnect", ValueBool: &newConfiguration.AllowReconnect},
		{Key: "nuclio.io/nats-jetstream-max-reconnect", ValueInt: &newConfiguration.MaxReconnect},
		{Key: "nuclio.io/nats-jetstream-reconnect-wait", ValueString: &newConfiguration.ReconnectWait},
		{Key: "nuclio.io/nats-jetstream-reconnect-jitter", ValueString: &newConfiguration.ReconnectJitter},
		{Key: "nuclio.io/nats-jetstream-timeout", ValueString: &newConfiguration.Timeout},
		{Key: "nuclio.io/nats-jetstream-drain-timeout", ValueString: &newConfiguration.DrainTimeout},
		{Key: "nuclio.io/nats-jetstream-flusher-timeout", ValueString: &newConfiguration.FlusherTimeout},
		{Key: "nuclio.io/nats-jetstream-ping-interval", ValueString: &newConfiguration.PingInterval},
		{Key: "nuclio.io/nats-jetstream-max-pings-out", ValueInt: &newConfiguration.MaxPingsOut},
	})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to populate configuration from annotations")
	}

	// set default values and parse durations
	if newConfiguration.MaxReconnect == 0 {
		newConfiguration.MaxReconnect = 60
	}

	if newConfiguration.MaxPingsOut == 0 {
		newConfiguration.MaxPingsOut = 2
	}

	for _, durationConfigField := range []trigger.DurationConfigField{
		{
			Name:    "reconnect wait",
			Value:   newConfiguration.ReconnectWait,
			Field:   &newConfiguration.reconnectWait,
			Default: 2 * time.Second,
		},
		{
			Name:    "reconnect jitter",
			Value:   newConfiguration.ReconnectJitter,
			Field:   &newConfiguration.reconnectJitter,
			Default: 100 * time.Millisecond,
		},
		{
			Name:    "timeout",
			Value:   newConfiguration.Timeout,
			Field:   &newConfiguration.timeout,
			Default: 2 * time.Second,
		},
		{
			Name:    "drain timeout",
			Value:   newConfiguration.DrainTimeout,
			Field:   &newConfiguration.drainTimeout,
			Default: 30 * time.Second,
		},
		{
			Name:    "flusher timeout",
			Value:   newConfiguration.FlusherTimeout,
			Field:   &newConfiguration.flusherTimeout,
			Default: 1 * time.Minute,
		},
		{
			Name:    "ping interval",
			Value:   newConfiguration.PingInterval,
			Field:   &newConfiguration.pingInterval,
			Default: 2 * time.Minute,
		},
	} {
		if err = newConfiguration.ParseDurationOrDefault(&durationConfigField); err != nil {
			return nil, err
		}
	}

	if newConfiguration.Stream == "" {
		return nil, errors.New("Stream must be set")
	}

	if newConfiguration.Consumer == "" {
		return nil, errors.New("Consumer must be set")
	}

	return &newConfiguration, nil
}

func (c *Configuration) GetNATSOptions() []nats.Option {
	var opts []nats.Option

	if !c.AllowReconnect {
		opts = append(opts, nats.NoReconnect())
	}

	opts = append(opts, nats.MaxReconnects(c.MaxReconnect))
	opts = append(opts, nats.ReconnectWait(c.reconnectWait))
	opts = append(opts, nats.ReconnectJitter(c.reconnectJitter, c.reconnectJitter*10)) // second argument is used if TLS is enabled
	opts = append(opts, nats.Timeout(c.timeout))
	opts = append(opts, nats.DrainTimeout(c.drainTimeout))
	opts = append(opts, nats.FlusherTimeout(c.flusherTimeout))
	opts = append(opts, nats.PingInterval(c.pingInterval))
	opts = append(opts, nats.MaxPingsOutstanding(c.MaxPingsOut))

	return opts
}
