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

package nucliozap

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mgutz/ansi"
	"github.com/nuclio/logger"
	"github.com/pavius/zap"
	"github.com/pavius/zap/zapcore"
)

type EncoderConfigJSON struct {
	LineEnding        string
	VarGroupName      string
	TimeFieldName     string
	TimeFieldEncoding string
}

type EncoderConfigConsole struct {
}

type EncoderConfig struct {
	JSON    EncoderConfigJSON
	Console EncoderConfigConsole
}

func NewEncoderConfig() *EncoderConfig {
	return &EncoderConfig{
		JSON: EncoderConfigJSON{
			LineEnding:        ",",
			TimeFieldName:     "time",
			TimeFieldEncoding: "epoch-millis",
		},
	}
}

// Level is logging levels
type Level int8

// Predefined logging levels
const (
	DebugLevel  Level = Level(zapcore.DebugLevel)
	InfoLevel   Level = Level(zapcore.InfoLevel)
	WarnLevel   Level = Level(zapcore.WarnLevel)
	ErrorLevel  Level = Level(zapcore.ErrorLevel)
	DPanicLevel Level = Level(zapcore.DPanicLevel)
	PanicLevel  Level = Level(zapcore.PanicLevel)
	FatalLevel  Level = Level(zapcore.FatalLevel)
)

type writerWrapper struct {
	io.Writer
}

func (w writerWrapper) Sync() error {
	return nil
}

// NuclioZap is a concrete implementation of the nuclio logger interface, using zap
type NuclioZap struct {
	*zap.SugaredLogger
	atomicLevel         zap.AtomicLevel
	coloredLevelDebug   string
	coloredLevelInfo    string
	coloredLevelWarn    string
	coloredLevelError   string
	colorLoggerName     func(string) string
	customEncoderConfig *EncoderConfig
	encoding            string
}

// NewNuclioZap create a configurable logger
func NewNuclioZap(name string,
	encoding string,
	customEncoderConfig *EncoderConfig,
	sink io.Writer,
	errSink io.Writer,
	level Level) (*NuclioZap, error) {
	newNuclioZap := &NuclioZap{
		atomicLevel:         zap.NewAtomicLevelAt(zapcore.Level(level)),
		customEncoderConfig: customEncoderConfig,
		encoding:            encoding,
	}

	if customEncoderConfig == nil {
		customEncoderConfig = NewEncoderConfig()
	}

	// create an encoder configuration
	encoderConfig := newNuclioZap.getEncoderConfig(encoding, customEncoderConfig)

	// create a sane configuration
	config := zap.Config{
		Level:              newNuclioZap.atomicLevel,
		Development:        true,
		Encoding:           encoding,
		EncoderConfig:      *encoderConfig,
		OutputWriters:      []zapcore.WriteSyncer{writerWrapper{sink}},
		ErrorOutputWriters: []zapcore.WriteSyncer{writerWrapper{errSink}},
		DisableStacktrace:  true,
	}

	newZapLogger, err := config.Build()
	if err != nil {
		return nil, err
	}

	newNuclioZap.SugaredLogger = newZapLogger.Sugar().Named(name)

	// initialize coloring by level
	newNuclioZap.initializeColors()

	return newNuclioZap, nil
}

// We use this istead of testing.Verbose since we don't want to get testing flags in our code
func isVerboseTesting() bool {
	for _, arg := range os.Args {
		if arg == "-test.v=true" || arg == "-test.v" {
			return true
		}
	}
	return false
}

// NewNuclioZapTest creates a logger pre-configured for tests
func NewNuclioZapTest(name string) (*NuclioZap, error) {
	var loggerLevel Level

	if isVerboseTesting() {
		loggerLevel = DebugLevel
	} else {
		loggerLevel = InfoLevel
	}

	return NewNuclioZapCmd(name, loggerLevel)
}

// NewNuclioZapCmd creates a logger pre-configured for commands
func NewNuclioZapCmd(name string, level Level) (*NuclioZap, error) {
	return NewNuclioZap(name, "console", nil, os.Stdout, os.Stdout, level)
}

// GetLevelByName return logging level by name
func GetLevelByName(levelName string) Level {
	switch levelName {
	case "info":
		return Level(zapcore.InfoLevel)
	case "warn":
		return Level(zapcore.WarnLevel)
	case "error":
		return Level(zapcore.ErrorLevel)
	case "dpanic":
		return Level(zapcore.DPanicLevel)
	case "panic":
		return Level(zapcore.PanicLevel)
	case "fatal":
		return Level(zapcore.FatalLevel)
	default:
		return Level(zapcore.DebugLevel)
	}
}

// SetLevel sets the logging level
func (nz *NuclioZap) SetLevel(level Level) {
	nz.atomicLevel.SetLevel(zapcore.Level(level))
}

// GetLevel returns the current logging level
func (nz *NuclioZap) GetLevel() Level {
	return Level(nz.atomicLevel.Level())
}

// Errors emits error level log
func (nz *NuclioZap) Error(format interface{}, vars ...interface{}) {
	formatString, formatIsString := format.(string)
	if formatIsString {
		nz.SugaredLogger.Errorf(formatString, vars...)
	} else {
		nz.SugaredLogger.Error(format)
	}
}

// ErrorCtx emits an unstructured debug log with context
func (nz *NuclioZap) ErrorCtx(ctx context.Context, format interface{}, vars ...interface{}) {
	nz.SugaredLogger.Errorw(nz.getFormatWithContext(ctx, format), nz.prepareVars(vars)...)
}

// ErrorWith emits error level log with arguments
func (nz *NuclioZap) ErrorWith(format interface{}, vars ...interface{}) {
	nz.SugaredLogger.Errorw(format.(string), vars...)
}

// ErrorWithCtx emits debug level log with arguments
func (nz *NuclioZap) ErrorWithCtx(ctx context.Context, format interface{}, vars ...interface{}) {
	nz.SugaredLogger.Errorw(format.(string), nz.addContextToVars(ctx, nz.prepareVars(vars))...)
}

// Warn emits warn level log
func (nz *NuclioZap) Warn(format interface{}, vars ...interface{}) {
	formatString, formatIsString := format.(string)
	if formatIsString {
		nz.SugaredLogger.Warnf(formatString, vars...)
	} else {
		nz.SugaredLogger.Warn(format)
	}
}

// WarnCtx emits an unstructured debug log with context
func (nz *NuclioZap) WarnCtx(ctx context.Context, format interface{}, vars ...interface{}) {
	nz.SugaredLogger.Warnw(nz.getFormatWithContext(ctx, format), nz.prepareVars(vars)...)
}

// WarnWith emits warn level log with arguments
func (nz *NuclioZap) WarnWith(format interface{}, vars ...interface{}) {
	nz.SugaredLogger.Warnw(format.(string), vars...)
}

// WarnWithCtx emits debug level log with arguments
func (nz *NuclioZap) WarnWithCtx(ctx context.Context, format interface{}, vars ...interface{}) {
	nz.SugaredLogger.Warnw(format.(string), nz.addContextToVars(ctx, nz.prepareVars(vars))...)
}

// Info emits info level log
func (nz *NuclioZap) Info(format interface{}, vars ...interface{}) {
	formatString, formatIsString := format.(string)
	if formatIsString {
		nz.SugaredLogger.Infof(formatString, vars...)
	} else {
		nz.SugaredLogger.Info(format)
	}
}

// InfoCtx emits an unstructured debug log with context
func (nz *NuclioZap) InfoCtx(ctx context.Context, format interface{}, vars ...interface{}) {
	nz.SugaredLogger.Infow(nz.getFormatWithContext(ctx, format), nz.prepareVars(vars)...)
}

// InfoWith emits info level log with arguments
func (nz *NuclioZap) InfoWith(format interface{}, vars ...interface{}) {
	nz.SugaredLogger.Infow(format.(string), nz.prepareVars(vars)...)
}

// InfoWithCtx emits debug level log with arguments
func (nz *NuclioZap) InfoWithCtx(ctx context.Context, format interface{}, vars ...interface{}) {
	nz.SugaredLogger.Infow(format.(string), nz.addContextToVars(ctx, nz.prepareVars(vars))...)
}

// Debug emits debug level log
func (nz *NuclioZap) Debug(format interface{}, vars ...interface{}) {
	formatString, formatIsString := format.(string)
	if formatIsString {
		nz.SugaredLogger.Debugf(formatString, vars...)
	} else {
		nz.SugaredLogger.Debug(format)
	}
}

// DebugCtx emits an unstructured debug log with context
func (nz *NuclioZap) DebugCtx(ctx context.Context, format interface{}, vars ...interface{}) {
	nz.SugaredLogger.Debugw(nz.getFormatWithContext(ctx, format), nz.prepareVars(vars)...)
}

// DebugWith emits debug level log with arguments
func (nz *NuclioZap) DebugWith(format interface{}, vars ...interface{}) {
	nz.SugaredLogger.Debugw(format.(string), nz.prepareVars(vars)...)
}

// DebugWithCtx emits debug level log with arguments
func (nz *NuclioZap) DebugWithCtx(ctx context.Context, format interface{}, vars ...interface{}) {
	nz.SugaredLogger.Debugw(format.(string), nz.addContextToVars(ctx, nz.prepareVars(vars))...)
}

// Flush flushes the log
func (nz *NuclioZap) Flush() {
	nz.Sync()
}

// GetChild returned a named child logger
func (nz *NuclioZap) GetChild(name string) logger.Logger {
	return &NuclioZap{
		SugaredLogger:       nz.Named(name),
		encoding:            nz.encoding,
		customEncoderConfig: nz.customEncoderConfig,
	}
}

func (nz *NuclioZap) encodeLoggerName(loggerName string, enc zapcore.PrimitiveArrayEncoder) {
	const maxLoggerNameLength = 25
	actualLoggerNameLength := len(loggerName)
	var encodedLoggerName string

	if actualLoggerNameLength >= maxLoggerNameLength {
		encodedLoggerName = loggerName[actualLoggerNameLength-maxLoggerNameLength:]

	} else {
		encodedLoggerName = strings.Repeat(" ", maxLoggerNameLength-actualLoggerNameLength) + loggerName
	}

	// just truncate
	enc.AppendString(nz.colorLoggerName(encodedLoggerName))
}

func (nz *NuclioZap) encodeStdoutLevel(level zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	switch level {
	case zapcore.InfoLevel:
		enc.AppendString(nz.coloredLevelInfo)
		return
	case zapcore.WarnLevel:
		enc.AppendString(nz.coloredLevelWarn)
		return
	case zapcore.ErrorLevel:
		enc.AppendString(nz.coloredLevelError)
		return
	}

	enc.AppendString(nz.coloredLevelDebug)
}

func (nz *NuclioZap) encodeStdoutTime(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("06.01.02 15:04:05.000"))
}

func (nz *NuclioZap) initializeColors() {
	nz.coloredLevelDebug = ansi.Color("(D)", "green")
	nz.coloredLevelInfo = ansi.Color("(I)", "blue")
	nz.coloredLevelWarn = ansi.Color("(W)", "yellow")
	nz.coloredLevelError = ansi.Color("(E)", "red")

	nz.colorLoggerName = ansi.ColorFunc("white")
}

func (nz *NuclioZap) getEncoderConfig(encoding string, encoderConfig *EncoderConfig) *zapcore.EncoderConfig {
	if encoding == "console" {
		return &zapcore.EncoderConfig{
			TimeKey:          "time",
			LevelKey:         "level",
			NameKey:          "name",
			CallerKey:        "",
			MessageKey:       "message",
			StacktraceKey:    "stack",
			LineEnding:       zapcore.DefaultLineEnding,
			EncodeLevel:      nz.encodeStdoutLevel,
			EncodeTime:       nz.encodeStdoutTime,
			EncodeDuration:   zapcore.StringDurationEncoder,
			EncodeCaller:     func(zapcore.EntryCaller, zapcore.PrimitiveArrayEncoder) {},
			EncodeLoggerName: nz.encodeLoggerName,
		}
	}

	var timeEncoder zapcore.TimeEncoder
	switch encoderConfig.JSON.TimeFieldEncoding {
	case "iso8601":
		timeEncoder = zapcore.ISO8601TimeEncoder
	default:
		timeEncoder = zapcore.EpochMillisTimeEncoder
	}

	return &zapcore.EncoderConfig{
		TimeKey:          encoderConfig.JSON.TimeFieldName,
		LevelKey:         "level",
		NameKey:          "name",
		CallerKey:        "",
		MessageKey:       "message",
		StacktraceKey:    "stack",
		LineEnding:       encoderConfig.JSON.LineEnding,
		EncodeLevel:      zapcore.LowercaseLevelEncoder,
		EncodeTime:       timeEncoder,
		EncodeDuration:   zapcore.SecondsDurationEncoder,
		EncodeCaller:     func(zapcore.EntryCaller, zapcore.PrimitiveArrayEncoder) {},
		EncodeLoggerName: zapcore.FullLoggerNameEncoder,
	}
}

func (nz *NuclioZap) addContextToVars(ctx context.Context, vars []interface{}) []interface{} {
	if ctx == nil {
		return vars
	}

	// get request ID from context
	requestID := ctx.Value("RequestID")

	// if not set, don't add it to vars
	if requestID == nil || requestID == "" {
		return vars
	}

	// create a slice 2 slots larger
	varsWithContext := make([]interface{}, 0, len(vars)+2)
	varsWithContext = append(varsWithContext, "requestID")
	varsWithContext = append(varsWithContext, requestID)
	varsWithContext = append(varsWithContext, vars...)

	return varsWithContext
}

func (nz *NuclioZap) getFormatWithContext(ctx context.Context, format interface{}) string {
	formatString := format.(string)

	// get request ID from context
	requestID := ctx.Value("RequestID")

	// if not set, don't add it to vars
	if requestID == nil || requestID == "" {
		return formatString
	}

	return formatString + fmt.Sprintf(" (requestID: %s)", requestID)
}

func (nz *NuclioZap) prepareVars(vars []interface{}) []interface{} {
	if nz.encoding != "json" || nz.customEncoderConfig == nil || nz.customEncoderConfig.JSON.VarGroupName == "" {
		return vars
	}

	// must be an even number of parameters
	if len(vars)&0x1 != 0 {
		panic("Odd number of logging vars - must be key/value")
	}

	formattedVars := ""

	// create key=value pairs
	for varIndex := 0; varIndex < len(vars); varIndex += 2 {
		formattedVars += fmt.Sprintf("%s=%+v || ", vars[varIndex], vars[varIndex+1])
	}

	// if nothing was created, don't generate a group
	if len(formattedVars) == 0 {
		return []interface{}{}
	}

	return []interface{}{
		nz.customEncoderConfig.JSON.VarGroupName,
		formattedVars[:len(formattedVars)-4],
	}
}
