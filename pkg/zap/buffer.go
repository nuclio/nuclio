package nucliozap

import (
	"bytes"
	"time"

	"github.com/pkg/errors"
)

var ErrBufferPoolAllocationTimeout = errors.New("Timed out waiting for buffer logger")

// a logger who outputs the records to a buffer
type BufferLogger struct {
	Logger *NuclioZap
	Writer *bytes.Buffer
}

func NewBufferLogger(name string, encoding string, level Level) (*BufferLogger, error) {
	writer := &bytes.Buffer{}

	// create a logger that is able to capture the output into a buffer. if a request arrives
	// and the user wishes to capture the log, this will be used as the logger instead of the default
	// logger
	newLogger, err := NewNuclioZap(name,
		encoding,
		writer,
		writer,
		level)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create buffer logger")
	}

	return &BufferLogger{
		Logger: newLogger,
		Writer: writer,
	}, nil
}

// a pool for buffer loggers
type BufferLoggerPool struct {
	bufferLoggerChan chan *BufferLogger
	defaultAllocateTimeout time.Duration
}

// a pool of buffer loggers
func NewBufferLoggerPool(numBufferLoggers int,
	name string,
	encoding string,
	level Level) (*BufferLoggerPool, error) {

	// create a channel for the buffer loggers
	bufferLoggersChan := make(chan *BufferLogger, numBufferLoggers)

	// create buffer loggers
	for bufferLoggerIdx := 0; bufferLoggerIdx < numBufferLoggers; bufferLoggerIdx++ {
		newBufferLogger, err := NewBufferLogger(name, encoding, level)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to create buffer logger")
		}

		// shove to channel
		bufferLoggersChan <- newBufferLogger
	}

	return &BufferLoggerPool{
		bufferLoggerChan: bufferLoggersChan,
		defaultAllocateTimeout: 10 * time.Second,
	}, nil
}

func (blp *BufferLoggerPool) Allocate(timeout *time.Duration) (*BufferLogger, error) {
	if timeout == nil {
		timeout = &blp.defaultAllocateTimeout
	}

	select {
	case bufferLogger := <-blp.bufferLoggerChan:

		// clear the buffer
		bufferLogger.Writer.Reset()

		return bufferLogger, nil
	case <-time.After(*timeout):
		return nil, ErrBufferPoolAllocationTimeout
	}
}

func (blp *BufferLoggerPool) Release(bufferLogger *BufferLogger) {
	blp.bufferLoggerChan <- bufferLogger
}
