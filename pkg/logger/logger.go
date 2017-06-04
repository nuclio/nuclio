package logger

type Fields map[string]interface{}

type Logger interface {

	// emit a log entry of a given verbosity. the first argument may be an object, a string
	// or a format string. in case of the latter, the following varargs are passed
	// to a formatter (e.g. fmt.Sprintf)
	Error(format interface{}, vars ...interface{})
	Warn(format interface{}, vars ...interface{})
	Info(format interface{}, vars ...interface{})
	Debug(format interface{}, vars ...interface{})

	// reports an error (as warning) and returns a new error, whose text is that
	// of the formatted string. this allows a sort of "call stack" of logs when
	// an error occurs
	Report(err error, format interface{}, vars ...interface{}) error

	// binds a field to a given logger, returning a temporary logger instance. this can
	// be used at the head of a function with a given context or more commonly to pass
	// structured variables to an emit. For example:
	//
	// l.With(logger.Fields{"f1": 0, "f2": "a"}).Debug("Structured")
	//
	// will provide the underlying logger a structured log entry, rather than one
	// whose variables are fused into the string (e.g. Debug("Structured %d %s", 0, "a")
	With(fields Fields) Logger

	// flushes buffered logs, if applicable
	Flush()

	// returns a child logger, if underlying logger supports hierarchal logging
	GetChild(name string) Logger
}
