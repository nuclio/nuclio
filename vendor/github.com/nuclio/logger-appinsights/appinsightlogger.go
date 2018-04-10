package appinsightslogger

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Microsoft/ApplicationInsights-Go/appinsights"
	"github.com/Microsoft/ApplicationInsights-Go/appinsights/contracts"
	"github.com/nuclio/logger"
)

type Logger struct {
	client appinsights.TelemetryClient
	name   string
}

func NewLogger(client appinsights.TelemetryClient, name string) (*Logger, error) {
	return &Logger{
		client: client,
		name: name,
	}, nil
}

func (logger *Logger) Close() error {
	logger.Flush()

	select {
	case <-logger.client.Channel().Close(10 * time.Second):
		return nil
	case <-time.After(30 * time.Second):
		return errors.New("timed out closing channel")
	}
}

// Error emits an unstructured error log
func (logger *Logger) Error(format interface{}, vars ...interface{}) {
	logger.emitUnstructured(appinsights.Error, format, vars...)
}

// Warn emits an unstructured warning log
func (logger *Logger) Warn(format interface{}, vars ...interface{}) {
	logger.emitUnstructured(appinsights.Warning, format, vars...)
}

// Info emits an unstructured informational log
func (logger *Logger) Info(format interface{}, vars ...interface{}) {
	logger.emitUnstructured(appinsights.Information, format, vars...)
}

// Debug emits an unstructured debug log
func (logger *Logger) Debug(format interface{}, vars ...interface{}) {

	// debug will use the *Verbose* severity level
	logger.emitUnstructured(appinsights.Verbose, format, vars...)
}

// ErrorWith emits a structured error log
func (logger *Logger) ErrorWith(format interface{}, vars ...interface{}) {
	logger.emitStructured(appinsights.Error, format, vars...)
}

// WarnWith emits a structured warning log
func (logger *Logger) WarnWith(format interface{}, vars ...interface{}) {
	logger.emitStructured(appinsights.Warning, format, vars...)
}

// InfoWith emits a structured info log
func (logger *Logger) InfoWith(format interface{}, vars ...interface{}) {
	logger.emitStructured(appinsights.Information, format, vars...)
}

// DebugWith emits a structured debug log
func (logger *Logger) DebugWith(format interface{}, vars ...interface{}) {
	logger.emitStructured(appinsights.Verbose, format, vars...)
}

// Flush flushes buffered logs
func (logger *Logger) Flush() {
	logger.client.Channel().Flush()
}

// GetChild returns a child logger
func (logger *Logger) GetChild(name string) logger.Logger {
	loggerInstance, _ := NewLogger(logger.client, fmt.Sprintf("%s.%s", logger.name, name))

	return loggerInstance
}

func toString(value interface{}) string {
	switch typedValue := value.(type) {
	case string:
		return typedValue
	case int:
		return strconv.Itoa(typedValue)
	case float64:
		return strconv.FormatFloat(typedValue, 'f', 6, 64)
	default:
		return fmt.Sprintf("%v", value)
	}
}

func (logger *Logger) emitUnstructured(severity contracts.SeverityLevel, format interface{}, vars ...interface{}) {
	message := fmt.Sprintf(toString(format), vars...)
	trace := appinsights.NewTraceTelemetry(message, severity)
	logger.client.Track(trace)
}

func (logger *Logger) emitStructured(severity contracts.SeverityLevel, message interface{}, vars ...interface{}) {
	trace := appinsights.NewTraceTelemetry(toString(message), severity)

	// set properties
	for varIdx := 0; varIdx < len(vars); varIdx += 2 {
		key := toString(vars[varIdx])
		value := toString(vars[varIdx + 1])

		trace.Properties[key] = value
	}

	logger.client.Track(trace)
}
