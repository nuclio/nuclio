package nucliozap

import (
	"strings"
	"time"

	"github.com/mgutz/ansi"
	"github.com/pavius/zap"
	"github.com/pavius/zap/zapcore"
)

type Level int8

const (
	DebugLevel Level = Level(zapcore.DebugLevel)
	InfoLevel Level = Level(zapcore.InfoLevel)
	WarnLevel Level = Level(zapcore.WarnLevel)
	ErrorLevel Level = Level(zapcore.ErrorLevel)
	DPanicLevel Level = Level(zapcore.DPanicLevel)
	PanicLevel Level = Level(zapcore.PanicLevel)
	FatalLevel Level = Level(zapcore.FatalLevel)
)

// concrete implementation of the nuclio logger interface, using zap
type NuclioZap struct {
	*zap.SugaredLogger
	coloredLevelDebug string
	coloredLevelInfo  string
	coloredLevelWarn  string
	coloredLevelError string
	colorLoggerName func(string) string
}

func NewNuclioZap(name string, level Level) (*NuclioZap, error) {
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
		Level:             zap.NewAtomicLevelAt(zapcore.Level(level)),
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
	newNuclioZap.initializeColors()

	return newNuclioZap, nil
}

func (nz *NuclioZap) Error(format interface{}, vars ...interface{}) {
	formatString, formatIsString := format.(string)
	if formatIsString {
		nz.SugaredLogger.Errorf(formatString, vars...)
	} else {
		nz.SugaredLogger.Error(format)
	}
}

func (nz *NuclioZap) ErrorWith(format interface{}, vars ...interface{}) {
	nz.SugaredLogger.Errorw(format.(string), vars...)
}

func (nz *NuclioZap) Warn(format interface{}, vars ...interface{}) {
	formatString, formatIsString := format.(string)
	if formatIsString {
		nz.SugaredLogger.Warnf(formatString, vars...)
	} else {
		nz.SugaredLogger.Warn(format)
	}
}

func (nz *NuclioZap) WarnWith(format interface{}, vars ...interface{}) {
	nz.SugaredLogger.Warnw(format.(string), vars...)
}

func (nz *NuclioZap) Info(format interface{}, vars ...interface{}) {
	formatString, formatIsString := format.(string)
	if formatIsString {
		nz.SugaredLogger.Infof(formatString, vars...)
	} else {
		nz.SugaredLogger.Info(format)
	}
}

func (nz *NuclioZap) InfoWith(format interface{}, vars ...interface{}) {
	nz.SugaredLogger.Infow(format.(string), vars...)
}

func (nz *NuclioZap) DebugWith(format interface{}, vars ...interface{}) {
	nz.SugaredLogger.Debugw(format.(string), vars...)
}

func (nz *NuclioZap) Debug(format interface{}, vars ...interface{}) {
	formatString, formatIsString := format.(string)
	if formatIsString {
		nz.SugaredLogger.Debugf(formatString, vars...)
	} else {
		nz.SugaredLogger.Debug(format)
	}
}

func (nz *NuclioZap) Flush() {
	nz.Sync()
}

func (nz *NuclioZap) GetChild(name string) interface{} {
	return &NuclioZap{SugaredLogger: nz.Named(name)}
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
