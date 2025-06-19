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
	"io"

	"github.com/nuclio/errors"
	"github.com/nuclio/nuclio-sdk-go"
)

type RpcLogRecord struct {
	DateTime string                 `json:"datetime"`
	Level    string                 `json:"level"`
	Message  string                 `json:"message"`
	With     map[string]interface{} `json:"with"`
}

type singleSerialisedResult struct {
	StatusCode   int                    `json:"status_code"`
	ContentType  string                 `json:"content_type"`
	Headers      map[string]interface{} `json:"headers"`
	EventId      string                 `json:"event_id"`
	Body         string                 `json:"body"`
	BodyEncoding string                 `json:"body_encoding"`
}

type StreamStart struct {
	*nuclio.ResponseStream
	Err error
	// firstChunk is the first chunk of data that was received
	// we can't write it to the nuclio.ResponseStream until nobody reads from it, because it's blocking operation
	// so it should be written to the stream only when we should block the execution
	firstChunk []byte
}

func NewStreamStart(responseStream *nuclio.ResponseStream) *StreamStart {
	return &StreamStart{ResponseStream: responseStream}
}

func newStreamStartFromSingleResult(singleResult *SingleResult) *StreamStart {
	responseStream := nuclio.NewResponseStream(singleResult.ContentType, singleResult.Headers, singleResult.StatusCode)
	return &StreamStart{
		ResponseStream: responseStream,
		firstChunk:     singleResult.Body,
	}
}

func (ss *StreamStart) WriteFirstChunk() error {
	_, err := ss.SendChunk(ss.firstChunk)
	return err
}
func (ss *StreamStart) IsStream() bool { return true }
func (ss *StreamStart) Error() error {
	return ss.Err
}

func (ss *StreamStart) GetProcessingResult() nuclio.ProcessingResult {
	return ss.ResponseStream
}

type StreamEnd struct{}

func (eos *StreamEnd) IsStream() bool { return false }

func (eos *StreamEnd) Error() error {
	return nil
}

type BodyOnly struct {
	Body []byte
	Err  error
}

func (b *BodyOnly) IsStream() bool { return false }

func (b *BodyOnly) Error() error {
	return b.Err
}

// NewBodyOnlyFromBase64 creates a BodyOnly result from base64-encoded data.
func NewBodyOnlyFromBase64(data []byte) Result {
	decoded, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return &BodyOnly{
			Err: errors.Errorf("failed to decode base64 body: %s", err.Error()),
		}
	}
	return &BodyOnly{
		Body: decoded,
	}
}

type SingleResult struct {
	*nuclio.Response
	EventId string `json:"event_id"`
	Err     error
}

func NewSingleResult(response *nuclio.Response) *SingleResult {
	if response == nil {
		response = &nuclio.Response{}
	}
	return &SingleResult{
		Response: response,
	}
}

func (sr *SingleResult) UnmarshalJSON(data []byte) error {
	var rawResult singleSerialisedResult
	if err := json.Unmarshal(data, &rawResult); err != nil {
		return err
	}

	// Decode body
	var decodedBody []byte
	switch rawResult.BodyEncoding {
	case "text":
		decodedBody = []byte(rawResult.Body)
	case "base64":
		var err error
		decodedBody, err = base64.StdEncoding.DecodeString(rawResult.Body)
		if err != nil {
			sr.Err = err
			return err
		}
	default:
		err := fmt.Errorf("unknown body encoding %q", rawResult.BodyEncoding)
		sr.Err = err
		return err
	}

	// Fill the Response
	if sr.Response == nil {
		sr.Response = &nuclio.Response{}
	}
	sr.Response.StatusCode = rawResult.StatusCode
	sr.Response.ContentType = rawResult.ContentType
	sr.Response.Headers = rawResult.Headers
	sr.Response.Body = decodedBody

	// Fill EventId
	sr.EventId = rawResult.EventId

	return nil
}

func NewSingleResultsWithError(err error) *SingleResult {
	singleResult := NewSingleResult(nil)
	singleResult.Err = err
	return singleResult
}

func (sr *SingleResult) IsStream() bool { return false }

func (sr *SingleResult) Error() error {
	return sr.Err
}

func (sr *SingleResult) GetProcessingResult() nuclio.ProcessingResult {
	return sr.Response
}

type BatchedResults struct {
	Results []*SingleResult
	Err     error
}

func (br *BatchedResults) IsStream() bool { return false }
func (br *BatchedResults) Error() error {
	return br.Err
}

func NewBatchedResults() *BatchedResults {
	return &BatchedResults{Results: make([]*SingleResult, 0)}
}

func NewBatchedResultsWithError(err error) *BatchedResults {
	return &BatchedResults{Err: err}
}

func NewResultFromData(data []byte) Result {
	switch data[0] {
	case 'r':
		// data[0] is a known prefix 'r'
		// data[1:] is expected to be a valid JSON object or array
		// so if data[1:] is of len 1, then it is not a valid result
		if len(data) < 2 {
			return NewSingleResultsWithError(errors.New("Data is too short to contain a valid result"))
		}

		if data[1] == '{' {
			var singleResult *SingleResult
			if err := json.Unmarshal(data[1:], &singleResult); err == nil {
				return singleResult
			}
		} else if data[1] == '[' {
			var results []*SingleResult
			if err := json.Unmarshal(data[1:], &results); err == nil {
				return &BatchedResults{Results: results}
			}
		}
		// Both failed, return result with error
		return NewSingleResultsWithError(fmt.Errorf("failed to unmarshal as single or batched result"))
	case 'e':
		return &StreamEnd{}
	case 'b':
		return NewBodyOnlyFromBase64(data[1:])
	case 'c':
		var singleResult *SingleResult
		if err := json.Unmarshal(data[1:], &singleResult); err != nil {
			singleResult.Err = errors.Wrap(err, "failed to unmarshal single result from stream start")
			return singleResult
		}
		return newStreamStartFromSingleResult(singleResult)
	default:
		return nil
	}
}

func NewResultWithNuclioProcessingResult(object interface{}) ResultWithNuclioProcessingResult {
	if object == nil {
		return NewSingleResult(nil)
	}
	switch typedResponse := object.(type) {
	case ResultWithNuclioProcessingResult:
		return typedResponse
	case *nuclio.ResponseStream:
		return NewStreamStart(typedResponse)
	case nuclio.ResponseStream:
		return NewStreamStart(&typedResponse)
	case *nuclio.Response:
		return NewSingleResult(typedResponse)
	case nuclio.Response:
		return NewSingleResult(&typedResponse)
	case io.ReadCloser:
		// if the response is an io.ReadCloser, create a response stream
		return NewStreamStart(
			nuclio.NewCustomResponseStream(
				"", nil, 0, typedResponse, nil),
		)
	case []byte:
		return NewSingleResult(&nuclio.Response{
			Body: typedResponse,
		})
	case string:
		return NewSingleResult(&nuclio.Response{
			Body: []byte(typedResponse),
		})
	default:
		// try to JSON-marshal the value
		if marshaled, marshalErr := json.Marshal(typedResponse); marshalErr == nil {
			return NewSingleResult(&nuclio.Response{Body: marshaled})
		}
		// fallback to string formatting if JSON marshalling fails
		return NewSingleResult(&nuclio.Response{Body: []byte(fmt.Sprintf("%v", typedResponse))})
	}
}
