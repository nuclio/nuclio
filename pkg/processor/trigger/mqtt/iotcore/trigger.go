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

package iotcoremqtt

import (
	"fmt"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/trigger/mqtt"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/golang-jwt/jwt/v4"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type iotcoremqtt struct {
	*mqtt.AbstractTrigger
	configuration *Configuration
}

func newTrigger(parentLogger logger.Logger,
	workerAllocator worker.Allocator,
	configuration *Configuration,
	restartTriggerChan chan trigger.Trigger) (trigger.Trigger, error) {

	newAbstractTrigger, err := mqtt.NewAbstractTrigger(parentLogger.GetChild("mqtt"),
		workerAllocator,
		&configuration.Configuration,
		restartTriggerChan)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create abstract trigger")
	}

	newIOTCoreMQTT := iotcoremqtt{
		AbstractTrigger: newAbstractTrigger,
		configuration:   configuration,
	}

	newIOTCoreMQTT.AbstractTrigger.Trigger = &newIOTCoreMQTT

	// set username to something so that client will send it
	newIOTCoreMQTT.configuration.Username = "ignored"

	// generate client ID
	newIOTCoreMQTT.configuration.ClientID = newIOTCoreMQTT.getClientID()

	return &newIOTCoreMQTT, nil
}

func (t *iotcoremqtt) Start(checkpoint functionconfig.Checkpoint) error {

	// generate new JWT and connect
	if err := t.connect(); err != nil {
		return errors.Wrap(err, "Failed to connect")
	}

	// wait 80% of the jwt refresh interval and refresh
	refreshInterval := time.Duration(float64(t.configuration.jwtRefreshInterval.Nanoseconds()) * 0.8)

	go func() {
		for {
			t.Logger.DebugWith("Waiting for JWT refresh", "duration", refreshInterval)

			time.Sleep(refreshInterval)

			// disconnect the client
			t.MQTTClient.Disconnect(100)

			// generate new JWT and connect
			if err := t.connect(); err != nil {
				t.Logger.WarnWith("Failed to connect", "err", err.Error())
			}
		}
	}()

	return nil
}

func (t *iotcoremqtt) createJWT(issuedAt time.Time) (string, error) {
	t.Logger.DebugWith("Creating JWT",
		"audience", t.configuration.ProjectID,
		"expiresIn", t.configuration.jwtRefreshInterval)

	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(t.configuration.PrivateKey.Contents))
	if err != nil {
		return "", errors.Wrap(err, "Failed to parse private key")
	}

	// sign payload with private key using sha-256
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.RegisteredClaims{
		Audience:  []string{t.configuration.ProjectID},
		ExpiresAt: jwt.NewNumericDate(issuedAt.Add(t.configuration.jwtRefreshInterval)),
		IssuedAt:  jwt.NewNumericDate(issuedAt),
	})

	return token.SignedString(privateKey)
}

func (t *iotcoremqtt) getClientID() string {
	return fmt.Sprintf("projects/%s/locations/%s/registries/%s/devices/%s",
		t.configuration.ProjectID,
		t.configuration.RegionName,
		t.configuration.RegistryID,
		t.configuration.DeviceID)
}

func (t *iotcoremqtt) connect() error {

	// create jwt for the next period
	signedJWTContents, err := t.createJWT(time.Now())
	if err != nil {
		return errors.Wrap(err, "Failed to create JWT")
	}

	// set the password
	t.configuration.Password = signedJWTContents

	// do the initial connect
	if err := t.Connect(); err != nil {
		return errors.Wrap(err, "Failed initial connect")
	}

	return nil
}
