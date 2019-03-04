package appinsightslogger

import (
	"context"
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
	level  logger.Level
}

func NewLogger(client appinsights.TelemetryClient, name string, level logger.Level) (*Logger, error) {
	return &Logger{
		client: client,
		name:   name,
		level:  level,
	}, nil
}

func (l *Logger) Close() error {
	l.Flush()

	select {
	case <-l.client.Channel().Close(10 * time.Second):
		return nil
	case <-time.After(30 * time.Second):
		return errors.New("timed out closing channel")
	}
}

// Error emits an unstructured error log
func (l *Logger) Error(format interface{}, vars ...interface{}) {
	if l.level <= logger.LevelError {
		l.emitUnstructured(appinsights.Error, format, vars...)
	}
}

// Warn emits an unstructured warning log
func (l *Logger) Warn(format interface{}, vars ...interface{}) {
	if l.level <= logger.LevelWarn {
		l.emitUnstructured(appinsights.Warning, format, vars...)
	}
}

// Info emits an unstructured informational log
func (l *Logger) Info(format interface{}, vars ...interface{}) {
	if l.level <= logger.LevelInfo {
		l.emitUnstructured(appinsights.Information, format, vars...)
	}
}

// Debug emits an unstructured debug log
func (l *Logger) Debug(format interface{}, vars ...interface{}) {

	// debug will use the *Verbose* severity level
	if l.level <= logger.LevelDebug {
		l.emitUnstructured(appinsights.Verbose, format, vars...)
	}
}

// ErrorWith emits a structured error log
func (l *Logger) ErrorWith(format interface{}, vars ...interface{}) {
	if l.level <= logger.LevelError {
		l.emitStructured(appinsights.Error, format, vars...)
	}
}

// WarnWith emits a structured warning log
func (l *Logger) WarnWith(format interface{}, vars ...interface{}) {
	if l.level <= logger.LevelWarn {
		l.emitStructured(appinsights.Warning, format, vars...)
	}
}

// InfoWith emits a structured info log
func (l *Logger) InfoWith(format interface{}, vars ...interface{}) {
	if l.level <= logger.LevelInfo {
		l.emitStructured(appinsights.Information, format, vars...)
	}
}

// DebugWith emits a structured debug log
func (l *Logger) DebugWith(format interface{}, vars ...interface{}) {
	if l.level <= logger.LevelDebug {
		l.emitStructured(appinsights.Verbose, format, vars...)
	}
}

// ErrorCtx emits an unstructred error log
func (l *Logger) ErrorCtx(ctx context.Context, format interface{}, vars ...interface{}) {
	if l.level <= logger.LevelError {
		vars = append(vars, ctx)
		l.emitUnstructured(appinsights.Error, format, vars...)
	}
}

// WarnCtx emits an unstructred error log
func (l *Logger) WarnCtx(ctx context.Context, format interface{}, vars ...interface{}) {
	if l.level <= logger.LevelWarn {
		vars = append(vars, ctx)
		l.emitUnstructured(appinsights.Warning, format, vars...)
	}
}

// InfoCtx emits an unstructred error log
func (l *Logger) InfoCtx(ctx context.Context, format interface{}, vars ...interface{}) {
	if l.level <= logger.LevelInfo {
		vars = append(vars, ctx)
		l.emitUnstructured(appinsights.Information, format, vars...)
	}
}

// DebugCtx emits an unstructred error log
func (l *Logger) DebugCtx(ctx context.Context, format interface{}, vars ...interface{}) {
	if l.level <= logger.LevelDebug {
		vars = append(vars, ctx)
		l.emitUnstructured(appinsights.Verbose, format, vars...)
	}
}

// ErrorWithCtx emits a structured error log
func (l *Logger) ErrorWithCtx(ctx context.Context, format interface{}, vars ...interface{}) {
	if l.level <= logger.LevelError {
		vars = append(vars, ctx)
		l.emitStructured(appinsights.Error, format, vars...)
	}
}

// WarnWithCtx emits a structured error log
func (l *Logger) WarnWithCtx(ctx context.Context, format interface{}, vars ...interface{}) {
	if l.level <= logger.LevelWarn {
		vars = append(vars, ctx)
		l.emitStructured(appinsights.Warning, format, vars...)
	}
}

// InfoWithCtx emits a structured error log
func (l *Logger) InfoWithCtx(ctx context.Context, format interface{}, vars ...interface{}) {
	if l.level <= logger.LevelInfo {
		vars = append(vars, ctx)
		l.emitStructured(appinsights.Information, format, vars...)
	}
}

// DebugWithCtx emits a structured error log
func (l *Logger) DebugWithCtx(ctx context.Context, format interface{}, vars ...interface{}) {
	if l.level <= logger.LevelDebug {
		vars = append(vars, ctx)
		l.emitStructured(appinsights.Verbose, format, vars...)
	}
}

// Flush flushes buffered logs
func (l *Logger) Flush() {
	l.client.Channel().Flush()
}

// GetChild returns a child logger
func (l *Logger) GetChild(name string) logger.Logger {
	loggerInstance, _ := NewLogger(l.client, fmt.Sprintf("%s.%s", l.name, name), l.level)

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

func (l *Logger) emitUnstructured(severity contracts.SeverityLevel, format interface{}, vars ...interface{}) {
	message := fmt.Sprintf(toString(format), vars...)
	trace := appinsights.NewTraceTelemetry(message, severity)
	l.client.Track(trace)
}

func (l *Logger) emitStructured(severity contracts.SeverityLevel, message interface{}, vars ...interface{}) {
	trace := appinsights.NewTraceTelemetry(toString(message), severity)

	// set properties
	for varIdx := 0; varIdx < len(vars); varIdx += 2 {
		key := toString(vars[varIdx])
		value := toString(vars[varIdx+1])

		trace.Properties[key] = value
	}

	l.client.Track(trace)
}
