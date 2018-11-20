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

package iotcoremqtt

import (
	"fmt"
	"time"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/trigger/mqtt"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/dgrijalva/jwt-go"
	"github.com/nuclio/logger"
)

type iotcoremqtt struct {
	*mqtt.AbstractTrigger
	configuration *Configuration
}

func newTrigger(parentLogger logger.Logger,
	workerAllocator worker.Allocator,
	configuration *Configuration) (trigger.Trigger, error) {

	newAbstractTrigger, err := mqtt.NewAbstractTrigger(parentLogger.GetChild("mqtt"), workerAllocator, &configuration.Configuration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create abstract trigger")
	}

	newIOTCoreMQTT := iotcoremqtt{
		AbstractTrigger: newAbstractTrigger,
		configuration:   configuration,
	}

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

func (t *iotcoremqtt) createJWT() (string, error) {
	t.Logger.DebugWith("Creating JWT", "expiresIn", t.configuration.jwtRefreshInterval)

	token := jwt.New(jwt.SigningMethodRS256)
	token.Claims = jwt.StandardClaims{
		Audience:  t.configuration.ProjectID,
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(t.configuration.jwtRefreshInterval).Unix(),
	}

	rsaPrivateKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(t.configuration.PrivateKey.Contents))
	if err != nil {
		return "", errors.Wrap(err, "Failed to parse RSA private key")
	}

	signedJWTContents, err := token.SignedString(rsaPrivateKey)
	if err != nil {
		return "", errors.Wrap(err, "Failed to sign JWT")
	}

	return signedJWTContents, nil
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
	signedJWTContents, err := t.createJWT()
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
