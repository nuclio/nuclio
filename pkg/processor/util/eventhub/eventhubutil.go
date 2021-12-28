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

package eventhubutil

import (
	"fmt"

	eventhubclient "github.com/Azure/go-amqp"
	"github.com/nuclio/errors"
)

func CreateSession(namespace string,
	sharedAccessKeyName string,
	sharedAccessKeyValue string) (*eventhubclient.Session, error) {

	// get eventhub URL
	eventhubURL := fmt.Sprintf("amqps://%s.servicebus.windows.net", namespace)
	auth := eventhubclient.ConnSASLPlain(sharedAccessKeyName, sharedAccessKeyValue)

	eventhubClient, err := eventhubclient.Dial(eventhubURL, auth)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to connect to eventhub @ %s", eventhubURL)
	}

	return eventhubClient.NewSession()
}
