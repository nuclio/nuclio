package formatted

import (
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/nuclio/nuclio/pkg/logger/formatted/formatter"
)

// Create log entry to file with json serializer
func newFileRotatedEntry(config *FileRotatedOutputConfig, outputFormatter logrus.Formatter) *logrus.Entry {

	logger := logrus.New()
	logger.Out = &lumberjack.Logger{
		Filename:   config.FullPath,
		MaxSize:    config.MaxFileSizeMB,
		MaxBackups: config.MaxNumFiles,
	}

	// default formatter is JSON
	if outputFormatter == nil {
		outputFormatter = &formatter.Json{}
	}

	logger.Formatter = outputFormatter

	return logrus.NewEntry(logger)
}

// Create log entry to file with json serializer
func newFileTimedEntry(config *FileTimedOutputConfig, outputFormatter logrus.Formatter) *logrus.Entry {

	// the lumberjack logger will rotate @ 10GB, though we hope no
	// services reach that size in the given timed rotation period
	lumberjackLogger := &lumberjack.Logger{
		Filename:   config.FullPath,
		MaxBackups: config.MaxNumFiles,
		MaxSize:    10 * 1024,
	}

	logger := logrus.New()
	logger.Out = lumberjackLogger

	// default formatter is JSON
	if outputFormatter == nil {
		outputFormatter = &formatter.Json{}
	}

	logger.Formatter = outputFormatter

	// every N seconds, rotate it
	go func() {
		for {
			time.Sleep(time.Duration(config.Period) * time.Second)
			lumberjackLogger.Rotate()
		}

	}()

	return logrus.NewEntry(logger)
}

// Create log entry to stdout with text serializer
func newStdoutEntry(config *StdoutOutputConfig, outputFormatter logrus.Formatter) *logrus.Entry {
	logger := logrus.New()
	logger.Out = os.Stdout

	// default formatter is JSON
	if outputFormatter == nil {
		outputFormatter = &formatter.HumanReadable{}
	}

	logger.Formatter = outputFormatter

	return logrus.NewEntry(logger)
}
