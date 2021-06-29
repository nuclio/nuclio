/*
Copyright 2021 The Nuclio Authors.

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

package logprocessing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/common"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

// PrettifyFunctionLogLine prettifies log line, and returns - (formattedLogLine, briefLogLine, error)
// when line shouldn't be added to brief error message - briefLogLine will be an empty string ("")
func PrettifyFunctionLogLine(logger logger.Logger, log []byte) (string, string, error) {
	var workerID, briefLogLine string

	functionLogLineInstance, err := CreateFunctionLogLine(log)
	if err != nil {
		return "", "", errors.Wrap(err, "Failed to create function log line")
	}

	// check required fields existence
	if functionLogLineInstance.Time == nil ||
		functionLogLineInstance.Level == nil ||
		functionLogLineInstance.Message == nil {
		return "", "", errors.New("Missing required fields in pod log line")
	}

	parsedTime, err := time.Parse(time.RFC3339, *functionLogLineInstance.Time)
	if err != nil {
		return "", "", err
	}

	logLevel := strings.ToUpper(*functionLogLineInstance.Level)[0]

	if functionLogLineInstance.With != nil {
		workerID = functionLogLineInstance.With["worker_id"]
	}

	// if worker ID wasn't explicitly given as an arg, try to infer worker ID from logger name
	if workerID == "" && functionLogLineInstance.Name != nil {
		workerID = inferWorkerID(*functionLogLineInstance.Name)
	}

	messageAndArgs := getMessageAndArgs(logger, functionLogLineInstance, log)

	res := fmt.Sprintf("[%s] (%c) %s", parsedTime.Format("15:04:05.000"), logLevel, messageAndArgs)

	if shouldAddToBriefErrorsMessage(logLevel, *functionLogLineInstance.Message, workerID) {
		briefLogLine = messageAndArgs
	}

	return res, briefLogLine, nil
}

func CreateFunctionLogLine(log []byte) (*FunctionLogLine, error) {
	functionLogLineInstance := &FunctionLogLine{}

	log = formatLogLine(log)

	if err := json.Unmarshal(log, &functionLogLineInstance); err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal log line")
	}

	// user function log lines has datetime and "with" fields
	// we will leverage these fields to differentiate between processor log lines
	// and function user log lines
	if functionLogLineInstance.Datetime != nil {

		// manipulate the time format so it can be parsed later
		unparsedTime := *functionLogLineInstance.Datetime + "Z"
		unparsedTime = strings.Replace(unparsedTime, " ", "T", 1)
		unparsedTime = strings.Replace(unparsedTime, ",", ".", 1)
		functionLogLineInstance.Time = &unparsedTime
	}

	// optional field for user function log lines
	if functionLogLineInstance.With != nil {
		more := createKeyValuePairs(functionLogLineInstance.With)
		functionLogLineInstance.More = &more
	}

	return functionLogLineInstance, nil

}

// InferWorkerID infers the worker ID from the logger name field
// e.g.: "processor.http.w5.python.logger" -> 5
func inferWorkerID(name string) string {
	processorRe := regexp.MustCompile(`^processor\..*\.w[0-9]+\..*`)
	if processorRe.MatchString(name) {
		splitName := strings.Split(name, ".")
		return splitName[2][1:]
	}

	return ""
}

func getMessageAndArgs(logger logger.Logger, functionLogLineInstance *FunctionLogLine, log []byte) string {

	var args string

	if functionLogLineInstance.More != nil {
		args = *functionLogLineInstance.More
	}

	var additionalKwargsAsString string

	additionalKwargs, err := getLogLineAdditionalKwargs(log)
	if err != nil {
		logger.WarnWith("Failed to get log line's additional kwargs",
			"logLineMessage", *functionLogLineInstance.Message)
	}
	additionalKwargsAsString = createKeyValuePairs(additionalKwargs)

	// format result depending on args/additional kwargs existence
	var messageArgsList []string
	if args != "" {
		messageArgsList = append(messageArgsList, args)
	}
	if additionalKwargsAsString != "" {
		messageArgsList = append(messageArgsList, additionalKwargsAsString)
	}
	if len(messageArgsList) > 0 {
		return fmt.Sprintf("%s [%s]", *functionLogLineInstance.Message, strings.Join(messageArgsList, " || "))
	}

	return *functionLogLineInstance.Message
}

func getLogLineAdditionalKwargs(log []byte) (map[string]string, error) {
	functionLogLineInstance, err := CreateFunctionLogLine(log)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create function log line")
	}

	// log as map may include more fields than function log line as user may add kwargs on log level
	// and not on "more"/"with"
	logAsMap := map[string]interface{}{}
	if err := json.Unmarshal(formatLogLine(log), &logAsMap); err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal log line")
	}

	additionalKwargs := map[string]string{}
	defaultArgs := functionLogLineInstance.GetJSONFields()

	// validate it is a suitable special arg
	for argKey, argValue := range logAsMap {

		// validate it is indeed an additional arg - it isn't a default arg
		if common.StringSliceContainsString(defaultArgs, argKey) {
			continue
		}

		// collect only strings
		switch v := argValue.(type) {
		case string:
			additionalKwargs[argKey] = v
		default:
			continue

			// TODO: uncomment when added unit / integration tests
			//case int, int8, int16, int32, int64:
			//	additionalKwargs[argKey] = fmt.Sprintf("%d", v)
			//case float32, float64:
			//	additionalKwargs[argKey] = fmt.Sprintf("%f", v)
		}
	}

	return additionalKwargs, nil
}

func shouldAddToBriefErrorsMessage(logLevel uint8, logMessage, workerID string) bool {
	knownFailureSubstrings := [...]string{"Failed to connect to broker"}
	ignoreFailureSubstrings := [...]string{
		string(common.UnexpectedTerminationChildProcess),
		string(common.FailedReadFromConnection),
	}

	// when the log message contains a failure that should be ignored
	for _, ignoreFailureSubstring := range ignoreFailureSubstrings {
		if strings.Contains(logMessage, ignoreFailureSubstring) {
			return false
		}
	}

	// show errors only of the first worker
	// done to prevent error duplication from several workers
	if workerID != "" && workerID != "0" {
		return false
	}
	// when log level is warning or above
	if logLevel != 'D' && logLevel != 'I' {
		return true
	}

	// when the log message contains a known failure substring
	for _, knownFailureSubstring := range knownFailureSubstrings {
		if strings.Contains(logMessage, knownFailureSubstring) {
			return true
		}
	}

	return false
}

func formatLogLine(log []byte) []byte {
	if isSDKLogLine(log) {
		return log[1:]
	}
	return log
}

func isSDKLogLine(log []byte) bool {
	return len(log) > 0 && log[0] == 'l'
}

// createKeyValuePairs creates a string from the given strings map by key and value (unordered)
// If the given map is nil return an empty string
// For example:
// input of (map[string]string{"a_key": "a_val", "b_key": "b_val"}) will return one of these:
// a_key="a_val" || b_key="b_val"
// b_key="b_val" || a_key="a_val"
func createKeyValuePairs(m map[string]string) string {
	b := new(bytes.Buffer)
	delimiter := " || "
	for key, value := range m {
		fmt.Fprintf(b, "%s=\"%s\"%s", key, value, delimiter) // nolint: errcheck
	}

	generatedString := b.String()

	if len(generatedString) != 0 {

		// remove last delimiter
		generatedString = generatedString[:len(generatedString)-len(delimiter)]
	}
	return generatedString
}
