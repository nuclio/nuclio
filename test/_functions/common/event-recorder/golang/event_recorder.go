package main

import (
	"encoding/json"
	"fmt"
	"github.com/nuclio/nuclio-sdk-go"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

const eventsLogFilePath = "/tmp/events.json"

func Handler(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
	triggerKind := ensureString(event.GetTriggerInfo().GetKind())
	if triggerKind != "http" || invokedByCron(event) {
		body := ensureString(event.GetBody())
		context.Logger.DebugWith("Received event",
			"body", body)
		// Serialize record
		record := map[string]interface{}{
			"body":      body,
			"headers":   ensureHeaders(event.GetHeaders()),
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		serializedRecord, err := json.Marshal(record)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize record: %w", err)
		}

		// Append record to log file
		file, err := os.OpenFile(eventsLogFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		defer file.Close()

		if _, err := file.WriteString(string(serializedRecord) + ", "); err != nil {
			return nil, fmt.Errorf("failed to write to log file: %w", err)
		}
		return nil, nil
	}

	// Read log file
	data, err := ioutil.ReadFile(eventsLogFilePath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to read log file: %w", err)
	}

	// Make valid JSON
	encodedEventLog := "[]"
	if len(data) > 2 {
		encodedEventLog = "[" + strings.TrimSuffix(string(data), ", ") + "]"
	}

	context.Logger.DebugWith("Returning events",
		"events", encodedEventLog)
	return encodedEventLog, nil
}

func invokedByCron(event nuclio.Event) bool {
	header := getHeader(event, "x-nuclio-invoke-trigger")
	return header == "cron"
}

func ensureString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		panic(fmt.Sprintf("unexpected type '%T'", value))
	}
}

func ensureHeaders(headers map[string]interface{}) map[string]string {
	ensuredHeaders := make(map[string]string)
	for key, value := range headers {
		ensuredHeaders[ensureString(key)] = ensureString(value)
	}
	return ensuredHeaders
}

func getHeader(event nuclio.Event, key string) string {
	headers := event.GetHeaders()
	if value, exists := headers[key]; exists {
		return ensureString(value)
	}
	return ""
}
