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

// PacketType represents a single-byte prefix that indicates the type of message
// being transmitted over the Nuclio processor connection.
type PacketType byte

const (
	PacketTypeSingleResponse PacketType = 'r'
	PacketTypeStreamStart    PacketType = 'c'
	PacketTypeBodyChunk      PacketType = 'b'
	PacketTypeEndOfStream    PacketType = 'e'
	PacketTypeMetrics        PacketType = 'm'
	PacketTypeLog            PacketType = 'l'
	PacketTypeStart          PacketType = 's'
)

type RpcLogRecord struct {
	DateTime string                 `json:"datetime"`
	Level    string                 `json:"level"`
	Message  string                 `json:"message"`
	With     map[string]interface{} `json:"with"`
}

// singleSerialisedResult is an internal struct used for deserialising a JSON response
// It matches the wire format and includes encoded body and metadata
type singleSerialisedResult struct {
	StatusCode   int                    `json:"status_code"`
	ContentType  string                 `json:"content_type"`
	Headers      map[string]interface{} `json:"headers"`
	EventId      string                 `json:"event_id"`
	Body         string                 `json:"body"`
	BodyEncoding string                 `json:"body_encoding"`
}

// StreamStart wraps a Nuclio ResponseStream and holds the first chunk to be sent later (when reader is ready)
type StreamStart struct {
	*nuclio.ResponseStream
	// firstChunk is the first chunk of data that was received
	// we can't write it to the nuclio.ResponseStream until somebody from it, because it's blocking operation
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
	return nil
}

func (ss *StreamStart) GetProcessingResult() nuclio.ProcessingResult {
	return ss.ResponseStream
}

// StreamEnd represents the logical end of a streaming response
type StreamEnd struct{}

func (eos *StreamEnd) IsStream() bool { return false }

func (eos *StreamEnd) Error() error {
	return nil
}

// BodyOnly represents a single body chunk (as []byte) without any metadata
// It may also carry an error if decoding failed
type BodyOnly struct {
	Body        []byte
	decodingErr error
}

func (b *BodyOnly) IsStream() bool { return false }

func (b *BodyOnly) Error() error {
	return b.decodingErr
}

// NewBodyOnlyFromBase64 creates a BodyOnly result from base64-encoded data.
func NewBodyOnlyFromBase64(data []byte) Result {
	decoded, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return &BodyOnly{
			decodingErr: errors.Errorf("failed to decode base64 body: %s", err.Error()),
		}
	}
	return &BodyOnly{
		Body: decoded,
	}
}

// SingleResult represents a full single response, either real or synthetic
// Includes a decoded body and optionally, an error
// may carry eventID when used in a batching flow
type SingleResult struct {
	*nuclio.Response
	EventId string `json:"event_id"`
	err     error
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

	decodedBody, err := sr.decodeBody(rawResult)

	if err != nil {
		return errors.Wrap(err, "Failed to decode body")
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

func (sr *SingleResult) decodeBody(rawResult singleSerialisedResult) ([]byte, error) {
	var decodedBody []byte
	switch rawResult.BodyEncoding {
	case "text":
		decodedBody = []byte(rawResult.Body)
	case "base64":
		var err error
		decodedBody, err = base64.StdEncoding.DecodeString(rawResult.Body)
		if err != nil {
			sr.err = err
			return nil, err
		}
	default:
		err := fmt.Errorf("unknown body encoding %q", rawResult.BodyEncoding)
		sr.err = err
		return nil, err
	}
	return decodedBody, nil
}

func NewSingleResultsWithError(err error) *SingleResult {
	singleResult := NewSingleResult(nil)
	singleResult.err = err
	return singleResult
}

func (sr *SingleResult) IsStream() bool { return false }

func (sr *SingleResult) Error() error {
	return sr.err
}

func (sr *SingleResult) GetProcessingResult() nuclio.ProcessingResult {
	return sr.Response
}

// BatchedResults contains a slice of multiple SingleResult items
// Used for transmitting batched (non-streaming) responses
type BatchedResults struct {
	Results []*SingleResult
	err     error
}

func (br *BatchedResults) IsStream() bool { return false }

func (br *BatchedResults) Error() error {
	return br.err
}

func NewBatchedResults() *BatchedResults {
	return &BatchedResults{Results: make([]*SingleResult, 0)}
}

func NewBatchedResultsWithError(err error) *BatchedResults {
	return &BatchedResults{err: err}
}

// NewResultFromData decodes raw incoming data into a typed `Result` object,
// based on the leading `PacketType` byte
func NewResultFromData(data []byte) Result {
	switch PacketType(data[0]) {
	case PacketTypeSingleResponse:
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
		return NewSingleResultsWithError(errors.New("failed to unmarshal as single or batched result"))
	case PacketTypeEndOfStream:
		return &StreamEnd{}
	case PacketTypeBodyChunk:
		return NewBodyOnlyFromBase64(data[1:])
	case PacketTypeStreamStart:
		var singleResult *SingleResult
		if err := json.Unmarshal(data[1:], &singleResult); err != nil {
			singleResult.err = errors.Wrap(err, "failed to unmarshal single result from stream start")
			return singleResult
		}
		return newStreamStartFromSingleResult(singleResult)
	default:
		return nil
	}
}

// NewResultWithProcessingResult wraps a generic input value into a `ResultWithProcessingResult`
func NewResultWithProcessingResult(object interface{}) ResultWithProcessingResult {
	if object == nil {
		return NewSingleResult(nil)
	}
	switch typedResponse := object.(type) {
	case ResultWithProcessingResult:
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

// NormalizeToResultWithProcessingResult ensures that the given `Result` is compatible
// with the `ResultWithProcessingResult` interface, converting it if necessary.
func NormalizeToResultWithProcessingResult(result Result) (ResultWithProcessingResult, error) {
	switch typedResult := result.(type) {
	case nil:
		return nil, nil
	case ResultWithProcessingResult:
		return typedResult, nil
	case *BodyOnly:
		single := NewSingleResult(&nuclio.Response{Body: typedResult.Body})
		single.err = typedResult.Error()
		return single, nil
	case *BatchedResults:
		if len(typedResult.Results) > 0 {
			return typedResult.Results[0], nil
		}
		return NewSingleResult(nil), nil
	default:
		return nil, errors.Errorf("cannot convert result of type %T to ResultWithProcessingResult", result)
	}
}

// NormalizeToBatchedResults ensures that a given `Result` is returned as a `*BatchedResults`.
// If it's a `SingleResult`, it wraps it in a batch. If already a batch, it's returned as-is.
func NormalizeToBatchedResults(result Result) (*BatchedResults, error) {
	switch typedResult := result.(type) {
	case *BatchedResults:
		return typedResult, nil
	case *SingleResult:
		return &BatchedResults{Results: []*SingleResult{typedResult}}, nil
	default:
		return nil, errors.Errorf("cannot convert result of type %T to BatchedResults", result)
	}
}
