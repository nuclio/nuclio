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

package main

import (
	ctx "context"
	"encoding/json"

	"github.com/Azure/go-amqp"
	"github.com/nuclio/nuclio-sdk-go"
)

type metric struct {
	ID                       string  `json:"id"`
	Latitude                 string  `json:"latitude"`
	Longitude                string  `json:"longitude"`
	TirePressure             float32 `json:"tirePressure"`
	FuelEfficiencyPercentage float32 `json:"fuelEfficiencyPercentage"`
	Temperature              int     `json:"temperature"`
	WeatherCondition         string  `json:"weatherCondition"`
}

type alarm struct {
	ID   string
	Type string
}

type weather struct {
	Temperature      int    `json:"temperature"`
	WeatherCondition string `json:"weatherCondition"`
}

func SensorHandler(context *nuclio.Context, event nuclio.Event) (interface{}, error) {

	// get alarms eventhub
	alarmsEventhub := context.DataBinding["alarmsEventhub"].(*amqp.Sender)

	// get enriched fleet eventhub
	enrichedFleetEventhub := context.DataBinding["enrichedFleetEventhub"].(*amqp.Sender)

	// unmarshal the eventhub metric
	eventHubMetric := metric{}
	if err := json.Unmarshal(event.GetBody(), &eventHubMetric); err != nil {
		return nil, err
	}

	// send alarm if tire pressure < threshold
	var MinTirePressureThreshold float32 = 2
	if eventHubMetric.TirePressure < MinTirePressureThreshold {
		marshaledAlarm, err := json.Marshal(alarm{ID: eventHubMetric.ID, Type: "LowTirePressue"})
		if err != nil {
			return nil, err
		}

		// send alarm to event hub
		if err := sendToEventHub(context, marshaledAlarm, alarmsEventhub); err != nil {
			return nil, err
		}
	}

	// prepare to send to spark via eventhub
	// call weather station for little enrichment
	temperature, weatherCondtion, err := getWeather(context, eventHubMetric)
	if err != nil {
		return nil, err
	}

	context.Logger.DebugWith("Got weather", "temp", temperature, "weather", weatherCondtion)

	// assign return values
	eventHubMetric.Temperature = temperature
	eventHubMetric.WeatherCondition = weatherCondtion

	// send to spark
	marshaledMetric, err := json.Marshal(eventHubMetric)
	if err != nil {
		return nil, err
	}

	if err := sendToEventHub(context, marshaledMetric, enrichedFleetEventhub); err != nil {
		return nil, err
	}

	return nil, nil
}

func sendToEventHub(context *nuclio.Context, data []byte, hub *amqp.Sender) error {

	// create an amqp message with the body
	message := amqp.Message{
		Data: [][]byte{data},
	}

	// send the metric
	if err := hub.Send(ctx.Background(), &message); err != nil {
		context.Logger.WarnWith("Failed to send message to eventhub", "err", err)

		return err
	}

	return nil
}

func getWeather(context *nuclio.Context, m metric) (int, string, error) {
	return 30, "stormy", nil
}
