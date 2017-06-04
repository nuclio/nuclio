package formatted

import (
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/nuclio/nuclio/pkg/logger"
)

type Formatted struct {
	entries []*logrus.Entry
}

func NewLogger(name string, outputConfigurations []interface{}) *Formatted {

	newFormattedLogger := &Formatted{
		entries: createEntriesFromOutputConfigurations(outputConfigurations),
	}

	return newFormattedLogger.With(logger.Fields{
		"who": name,
	}).(*Formatted)
}

func createEntriesFromOutputConfigurations(outputConfigurations []interface{}) []*logrus.Entry {
	entries := []*logrus.Entry{}

	for _, outputConfiguration := range outputConfigurations {
		var entry *logrus.Entry

		switch config := outputConfiguration.(type) {
		default:
			break
		case *StdoutOutputConfig:
			entry = newStdoutEntry(config, nil)
		case *FileRotatedOutputConfig:
			entry = newFileRotatedEntry(config, nil)
		case *FileTimedOutputConfig:
			entry = newFileTimedEntry(config, nil)
		}

		entries = append(entries, entry)
	}

	return entries
}

func (f *Formatted) SetLevel(level Level) {
	for _, entry := range f.entries {
		entry.Logger.Level = logrus.Level(level)
	}
}

// Returns a Logger with the given Fields. Calling any log method (e.g. Debug()) on the result
// will cause the given message to also include parameterized field data.
// Proxies a call to *logrus.Entry.WithFields, converting the given Fields to logrus.Fields as required.
func (f *Formatted) With(fields logger.Fields) logger.Logger {
	logrusFields := logrus.Fields(fields)
	entries := make([]*logrus.Entry, len(f.entries))

	for index, entry := range f.entries {
		entries[index] = entry.WithFields(logrusFields)
	}

	return &Formatted{entries: entries}
}

// Creates a new child Logger from an existing parent.
// Meant for use inside service/client constructors, with the resulting child being assigned as their Logger.
// Example: api.NewService() receives a client with the name "api" and calls GetChild on it with "service".
// The result is a child client with the name "api.service", and that is assigned as the api.Service's logger.
func (f *Formatted) GetChild(name string) logger.Logger {

	currentName := "default"
	if len(f.entries) != 0 {
		currentName = f.entries[0].Data["who"].(string)
	}
	// Since all entries share the same name, we can use the first one's name safely
	newName := fmt.Sprintf("%s.%s", currentName, name)
	return f.With(logger.Fields{"who": newName})
}

func (f *Formatted) Report(err error, format interface{}, vars ...interface{}) error {
	for _, entry := range f.entries {
		entry.WithError(err).Warn(format.(string))
	}

	return errors.New(format.(string))
}

func (f *Formatted) Debug(format interface{}, vars ...interface{}) {
	f.emitOnEntries(format.(string), Debug)
}

func (f *Formatted) Info(format interface{}, vars ...interface{}) {
	f.emitOnEntries(format.(string), Info)
}

func (f *Formatted) Warn(format interface{}, vars ...interface{}) {
	f.emitOnEntries(format.(string), Warn)
}

func (f *Formatted) Error(format interface{}, vars ...interface{}) {
	f.emitOnEntries(format.(string), Error)
}

func (f *Formatted) Flush() {
}

func (f *Formatted) emitOnEntries(message string, level Level) {
	var entryLogFunc func(...interface{})

	for _, entry := range f.entries {

		// Select logger method based on the given level
		switch level {
		case Debug:
			entryLogFunc = entry.Debug
		case Error:
			entryLogFunc = entry.Error
		case Info:
			entryLogFunc = entry.Info
		case Warn:
			entryLogFunc = entry.Warn
		}

		entryLogFunc(message)
	}
}
