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

//
// Accept a string (event.body) and scan for compliance using a list of regex patterns (SSN, Credit Cards, ..)
// will return a list of compliance violations in Json or "Passed"
// demonstrate the use of structured and unstructured log with different levels
// can be extended to write results to a stream/object storage
//

package main

import (
	"encoding/json"
	"regexp"

	"github.com/nuclio/nuclio-sdk-go"
)

// list of regular expression filters
var rx = map[string]*regexp.Regexp{
	"SSN":         regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
	"Credit card": regexp.MustCompile(`\b(?:\d[ -]*?){13,16}\b`)}

func Handler(context *nuclio.Context, event nuclio.Event) (interface{}, error) {

	// Unstructured debug message
	context.Logger.Debug("Process document %s, length %d", event.GetPath(), event.GetSize())

	data := string(event.GetBody())
	matchList := []string{}

	// Test content against a list of RegEx filters
	for k, v := range rx {
		if v.MatchString(string(data)) {
			matchList = append(matchList, "Contains "+k)
		}
	}

	// If we found a filter match add structured warning log message and respond with match list
	if len(matchList) > 0 {
		context.Logger.WarnWith("Document content warning", "path", event.GetPath(), "content", matchList)
		return json.Marshal(matchList)
	}

	return "Passed", nil
}
