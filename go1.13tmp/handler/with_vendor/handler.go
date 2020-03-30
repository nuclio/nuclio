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
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/nuclio/errors"
	"github.com/InVisionApp/tabular"
)

func Handler(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
	if event.GetContentType() == "" {
		return "", errors.New("Error!")
	}
	tab := tabular.New()
	tab.ColRJ("id", "ID", 6)
	tab.Col("env", "Environment", 14)
	tab.Col("cls", "Cluster", 10)
	tab.Col("svc", "Service", 25)
	tab.Col("hst", "Database Host", 25)
	tab.ColRJ("pct", "%CPU", 5)
	text := tab.Print("id", "env", "cls")
	return text, nil
}
