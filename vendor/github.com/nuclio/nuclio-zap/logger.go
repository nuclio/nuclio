package nuclio_zap

import (
	"strings"
	"time"

	"github.com/mgutz/ansi"
	"github.com/pavius/zap"
	"github.com/pavius/zap/zapcore"
)

// concrete implementation of the nuclio logger interface, using zap
type NuclioZap struct {
	*zap.SugaredLogger
	coloredLevelDebug string
	coloredLevelInfo  string
	coloredLevelWarn  string
	coloredLevelError string
}

func NewNuclioZap(name string) (*NuclioZap, error) {
	newNuclioZap := &NuclioZap{}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:          "time",
		LevelKey:         "level",
		NameKey:          "name",
		CallerKey:        "caller",
		MessageKey:       "message",
		StacktraceKey:    "stack",
		LineEnding:       zapcore.DefaultLineEnding,
		EncodeLevel:      newNuclioZap.encodeStdoutLevel,
		EncodeTime:       newNuclioZap.encodeStdoutTime,
		EncodeDuration:   zapcore.StringDurationEncoder,
		EncodeCaller:     func(zapcore.EntryCaller, zapcore.PrimitiveArrayEncoder) {},
		EncodeLoggerName: newNuclioZap.encodeLoggerName,
	}

	// create a sane configuration
	config := zap.Config{
		Level:             zap.NewAtomicLevelAt(zap.DebugLevel),
		Development:       true,
		Encoding:          "console",
		EncoderConfig:     encoderConfig,
		OutputPaths:       []string{"stdout"},
		ErrorOutputPaths:  []string{"stdout"},
		DisableStacktrace: true,
	}

	newZapLogger, err := config.Build()
	if err != nil {
		return nil, err
	}

	newNuclioZap.SugaredLogger = newZapLogger.Sugar().Named(name)

	// initialize coloring by level
	newNuclioZap.initializeColoredLevels()

	return newNuclioZap, nil
}

func (nz *NuclioZap) Error(format interface{}, vars ...interface{}) {
	formatString, formatIsString := format.(string)
	if formatIsString {
		nz.Errorf(formatString, vars...)
	} else {
		nz.Error(format, vars...)
	}
}

func (nz *NuclioZap) ErrorWith(format interface{}, vars ...interface{}) {
	nz.Errorw(format.(string), vars...)
}

func (nz *NuclioZap) Warn(format interface{}, vars ...interface{}) {
	formatString, formatIsString := format.(string)
	if formatIsString {
		nz.Warnf(formatString, vars...)
	} else {
		nz.Warn(format, vars...)
	}
}

func (nz *NuclioZap) WarnWith(format interface{}, vars ...interface{}) {
	nz.Warnw(format.(string), vars...)
}

func (nz *NuclioZap) Info(format interface{}, vars ...interface{}) {
	formatString, formatIsString := format.(string)
	if formatIsString {
		nz.Infof(formatString, vars...)
	} else {
		nz.Info(format, vars...)
	}
}

func (nz *NuclioZap) InfoWith(format interface{}, vars ...interface{}) {
	nz.Infow(format.(string), vars...)
}

func (nz *NuclioZap) DebugWith(format interface{}, vars ...interface{}) {
	nz.Debugw(format.(string), vars...)
}

func (nz *NuclioZap) Debug(format interface{}, vars ...interface{}) {
	formatString, formatIsString := format.(string)
	if formatIsString {
		nz.Debugf(formatString, vars...)
	} else {
		nz.Debug(format, vars...)
	}
}

func (nz *NuclioZap) Flush() {
	nz.Sync()
}

func (nz *NuclioZap) GetChild(name string) interface{} {
	return &NuclioZap{nz.Named(name), "", "", "", ""}
}

func (nz *NuclioZap) encodeLoggerName(loggerName string, enc zapcore.PrimitiveArrayEncoder) {
	const maxLoggerNameLength = 25
	actualLoggerNameLength := len(loggerName)

	if actualLoggerNameLength >= maxLoggerNameLength {

		// just truncate
		enc.AppendString(loggerName[actualLoggerNameLength-maxLoggerNameLength:])

	} else {

		// left pad with spaces
		enc.AppendString(strings.Repeat(" ", maxLoggerNameLength-actualLoggerNameLength-1))
		enc.AppendString(loggerName)
	}
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

func (nz *NuclioZap) initializeColoredLevels() {
	nz.coloredLevelDebug = ansi.Color("(D)", "green")
	nz.coloredLevelInfo = ansi.Color("(I)", "blue")
	nz.coloredLevelWarn = ansi.Color("(W)", "yellow")
	nz.coloredLevelError = ansi.Color("(E)", "red")
}
