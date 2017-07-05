package test

import (
	"testing"

	"github.com/nuclio/nuclio-zap"
)

type Logger interface {

	// emit a log entry of a given verbosity. the first argument may be an object, a string
	// or a format string. in case of the latter, the following varargs are passed
	// to a formatter (e.g. fmt.Sprintf)
	Error(format interface{}, vars ...interface{})
	ErrorWith(format interface{}, vars ...interface{})
	Warn(format interface{}, vars ...interface{})
	WarnWith(format interface{}, vars ...interface{})
	Info(format interface{}, vars ...interface{})
	InfoWith(format interface{}, vars ...interface{})
	Debug(format interface{}, vars ...interface{})
	DebugWith(format interface{}, vars ...interface{})

	// flushes buffered logs, if applicable
	Flush()

	// returns a child logger, if underlying logger supports hierarchal logging
	GetChild(name string) interface{}
}

func TestSimpleLogging(t *testing.T) {
	var logger Logger

	logger, err := nucliozap.NewNuclioZap("test")
	if err != nil {
		t.Error(err)
	}

	logger.Debug("Hello there")
	logger.Info("Hello there info")
	logger.Warn("Hello there %s", "warning")
	logger.DebugWith("Hello there", "a", 3, "b", "foo")
	logger.Error("An error %s %d", "aaa", 30)

	childLogger1 := logger.GetChild("child1").(Logger)
	childLogger1.DebugWith("What", "a", 1)

	childLogger2 := childLogger1.GetChild("child2").(Logger)
	childLogger2.DebugWith("Foo", "a", 1)
	childLogger2.DebugWith("Foo", "a", 1)
}
