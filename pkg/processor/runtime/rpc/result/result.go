/*
Copyright 2024 The Nuclio Authors.

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

package result

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/nuclio/logger"
)

type RpcLogRecord struct {
	DateTime string                 `json:"datetime"`
	Level    string                 `json:"level"`
	Message  string                 `json:"message"`
	With     map[string]interface{} `json:"with"`
}

type Result struct {
	StatusCode   int                    `json:"status_code"`
	ContentType  string                 `json:"content_type"`
	Body         string                 `json:"body"`
	BodyEncoding string                 `json:"body_encoding"`
	Headers      map[string]interface{} `json:"headers"`
	EventId      string                 `json:"event_id"`

	DecodedBody []byte
	Err         error
}

type BatchedResults struct {
	Results []*Result
	Err     error
}

func NewBatchedResults() *BatchedResults {
	return &BatchedResults{Results: make([]*Result, 0)}
}

func (br *BatchedResults) UnmarshalResponseData(logger logger.Logger, data []byte) {
	var results []*Result

	// define method to process a single result
	handleSingleUnmarshalledResult := func(unmarshalledResult *Result) {
		switch unmarshalledResult.BodyEncoding {
		case "text":
			unmarshalledResult.DecodedBody = []byte(unmarshalledResult.Body)
		case "base64":
			unmarshalledResult.DecodedBody, br.Err = base64.StdEncoding.DecodeString(unmarshalledResult.Body)
		default:
			unmarshalledResult.Err = fmt.Errorf("Unknown body encoding - %q", unmarshalledResult.BodyEncoding)
		}
	}

	if br.Err = json.Unmarshal(data, &results); br.Err != nil {
		// try to unmarshall data as a single result
		var singleResult *Result
		if br.Err = json.Unmarshal(data, &singleResult); br.Err != nil {
			logger.DebugWith("Failed to unmarshal result",
				"err", br.Err.Error())
			return
		} else {
			handleSingleUnmarshalledResult(singleResult)
			br.Results = append(br.Results, singleResult)
			return
		}
	}

	br.Results = results
	for _, unmarshalledResult := range br.Results {
		handleSingleUnmarshalledResult(unmarshalledResult)
	}
}
