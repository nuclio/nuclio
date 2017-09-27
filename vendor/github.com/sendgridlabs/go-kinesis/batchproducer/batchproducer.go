package batchproducer

import (
	"errors"
	"log"
	"os"
	"sync"
	"time"

	"github.com/sendgridlabs/go-kinesis"
)

// MaxKinesisBatchSize is the maximum number of records that Kinesis accepts in a request
const MaxKinesisBatchSize = 500

// Producer collects records individually and then sends them to Kinesis in
// batches in the background using PutRecords, with retries.
// A Producer will do nothing until Start is called.
type Producer interface {
	// Start starts the main goroutine. No need to call it using `go`.
	Start() error

	// Stop signals the main goroutine to finish. Once this is called, Add will immediately start
	// returning errors (unless and until Start is called again).
	Stop() error

	// Add might block if the BatchProducer has a buffer and the buffer is full.
	// In order to prevent filling the buffer and eventually blocking indefinitely,
	// Add will fail and return an error if the BatchProducer is stopped or stopping. Note
	// that it’s critical to check the return value because the BatchProducer could have
	// died in the background due to a panic (or something).
	Add(data []byte, partitionKey string) error

	// Flush stops the Producer using Stop and attempts to send all buffered records to Kinesis as
	// fast as possible with batches of size 500 (the maximum). It blocks until either all records
	// are sent or the timeout expires. It returns the number of records still remaining in the
	// buffer or (possibly) an error. (It doesn’t currently return errors but that is in the
	// signature for future-proofing.) A timeout value of 0 means no timeout.
	// If Flush finishes sending all records without timing out, and sendStats is true, it will
	// cause a single final StatsBatch to be sent to the StatsReceiver in Config, if set.
	Flush(timeout time.Duration, sendStats bool) (sent int, remaining int, err error)
}

// StatReceiver defines an object that can accept stats.
type StatReceiver interface {
	// Receive will be called by the main Producer goroutine so it will block all batches from being
	// sent, so make sure it is either very fast or never blocks at all!
	Receive(StatsBatch)
}

// StatsBatch is a kind of a snapshot of activity and happenings. Some of its fields represent
// "moment-in-time" values e.g. BufferSize is the size of the buffer at the moment the StatsBatch
// is sent. Other fields are cumulative since the last StatsBatch, i.e. ErrorsSinceLastStat.
type StatsBatch struct {
	// Moment-in-time stats
	BufferSize int

	// Cumulative stats
	KinesisErrorsSinceLastStat           int
	RecordsSentSuccessfullySinceLastStat int
	RecordsDroppedSinceLastStat          int
}

// BatchingKinesisClient is a subset of KinesisClient to ease mocking.
type BatchingKinesisClient interface {
	PutRecords(args *kinesis.RequestArgs) (resp *kinesis.PutRecordsResp, err error)
}

type BatchProducerLogger interface {
	Printf(format string, args ...interface{})
}

// Config is a collection of config values for a Producer
type Config struct {
	// AddBlocksWhenBufferFull controls the behavior of Add when the buffer is full. If true, Add
	// will block. If false, Add will return an error. This enables integrating applications to
	// decide how they want to handle a full buffer e.g. so they can discard records if there’s
	// a problem.
	AddBlocksWhenBufferFull bool

	// BatchSize controls the maximum size of the batches sent to Kinesis. If the number of records
	// in the buffer hits this size, a batch of this size will be sent at that time, regardless of
	// whether FlushInterval has a value or not.
	BatchSize int

	// BufferSize is the size of the buffer that stores records before they are sent to the Kinesis
	// stream. If when Add is called the number of records in the buffer is >= bufferSize then
	// Add will either block or return an error, depending on the value of AddBlocksWhenBufferFull.
	BufferSize int

	// FlushInterval controls how often the buffer is flushed to Kinesis. If nonzero, then every
	// time this interval occurs, if there are any records in the buffer, they will be flushed,
	// no matter how few there are. The size of the batch that’s flushed may be as small as 1 but
	// will be no larger than BatchSize.
	FlushInterval time.Duration

	// The logger used by the Producer.
	Logger BatchProducerLogger

	// MaxAttemptsPerRecord defines how many attempts should be made for each record before it is
	// dropped. You probably want this higher than the init default of 0.
	MaxAttemptsPerRecord int

	// StatInterval will be used to make a *best effort* attempt to send stats *approximately*
	// when this interval elapses. There’s no guarantee, however, since the main goroutine is
	// used to send the stats and therefore there may be some skew.
	StatInterval time.Duration

	// StatReceiver will have its Receive method called approximately every StatInterval.
	StatReceiver StatReceiver
}

// DefaultConfig is provided for convenience; if you have no specific preferences on how you’d
// like to configure your Producer you can pass this into New. The default value of Logger is
// the same as the standard logger in "log" : `log.New(os.Stderr, "", log.LstdFlags)`.
var DefaultConfig = Config{
	AddBlocksWhenBufferFull: false,
	BufferSize:              10000,
	FlushInterval:           1 * time.Second,
	BatchSize:               10,
	MaxAttemptsPerRecord:    10,
	StatInterval:            1 * time.Second,
	Logger:                  log.New(os.Stderr, "", log.LstdFlags),
}

var (
	// ErrAlreadyStarted is returned by Start if the Producer is already started.
	ErrAlreadyStarted = errors.New("already started")

	// ErrAlreadyStopped is returned by Stop if the Producer is already stopped.
	ErrAlreadyStopped = errors.New("already stopped")
)

// New creates and returns a BatchProducer that will do nothing until its Start method is called.
// Once it is started, it will flush a batch to Kinesis whenever either
// the flushInterval occurs (if flushInterval > 0) or the batchSize is reached,
// whichever happens first.
func New(
	client BatchingKinesisClient,
	streamName string,
	config Config,
) (Producer, error) {
	if config.BatchSize < 1 || config.BatchSize > MaxKinesisBatchSize {
		return nil, errors.New("BatchSize must be between 1 and 500 inclusive")
	}

	if config.BufferSize < config.BatchSize && config.FlushInterval <= 0 {
		return nil, errors.New("if BufferSize < BatchSize && FlushInterval <= 0 then the buffer will eventually fill up and Add will block forever")
	}

	if config.FlushInterval > 0 && config.FlushInterval < 50*time.Millisecond {
		return nil, errors.New("are you crazy")
	}

	batchProducer := batchProducer{
		client:      client,
		streamName:  streamName,
		config:      config,
		logger:      config.Logger,
		currentStat: new(StatsBatch),
		records:     make(chan batchRecord, config.BufferSize),
		start:       make(chan interface{}),
		stop:        make(chan interface{}),
	}

	return &batchProducer, nil
}

type batchProducer struct {
	client            BatchingKinesisClient
	streamName        string
	config            Config
	logger            BatchProducerLogger
	running           bool
	runningMu         sync.RWMutex
	consecutiveErrors int
	currentDelay      time.Duration
	currentStat       *StatsBatch
	records           chan batchRecord

	// start and stop will be unbuffered and will be used to send signals to start/stop and
	// response signals that indicate that the respective operations have completed.
	start chan interface{}
	stop  chan interface{}
}

type batchRecord struct {
	data         []byte
	partitionKey string
	sendAttempts int
}

// from/for interface Producer
func (b *batchProducer) Add(data []byte, partitionKey string) error {
	if !b.isRunning() {
		return errors.New("Cannot call Add when BatchProducer is not running (to prevent the buffer filling up and Add blocking indefinitely).")
	}
	if b.isBufferFull() && !b.config.AddBlocksWhenBufferFull {
		return errors.New("Buffer is full")
	}
	b.records <- batchRecord{data: data, partitionKey: partitionKey}
	return nil
}

// from/for interface Producer
func (b *batchProducer) Start() error {
	b.runningMu.Lock()
	defer b.runningMu.Unlock()

	if b.running {
		return ErrAlreadyStarted
	}

	go b.run()

	// We want run to run in the background (in a goroutine) but we don’t want to return until that
	// goroutine has actually entered its main loop. So we read from this non-buffered channel, which
	// will block until run writes a value to it.
	<-b.start

	b.running = true

	return nil
}

func (b *batchProducer) run() {
	flushTicker := &time.Ticker{}
	if b.config.FlushInterval > 0 {
		flushTicker = time.NewTicker(b.config.FlushInterval)
		defer flushTicker.Stop()
	}

	statTicker := &time.Ticker{}
	if b.config.StatReceiver != nil && b.config.StatInterval > 0 {
		statTicker = time.NewTicker(b.config.StatInterval)
		defer statTicker.Stop()
	}

	// used to signal Start that we are now running (entering the main loop)
	b.start <- true

	for {
		select {
		case <-flushTicker.C:
			b.sendBatch(b.config.BatchSize)
		case <-statTicker.C:
			b.sendStats()
		case <-b.stop:
			b.sendStats()
			b.stop <- true
			return
		default:
			if len(b.records) >= b.config.BatchSize {
				b.sendBatch(b.config.BatchSize)
			} else {
				time.Sleep(1 * time.Millisecond)
			}
		}
	}
}

// from/for interface Producer
func (b *batchProducer) Stop() error {
	b.runningMu.Lock()
	defer b.runningMu.Unlock()

	if !b.running {
		return ErrAlreadyStopped
	}

	// request the main goroutine to stop
	b.stop <- true

	// block until the main goroutine returns a value indicating that it has stopped
	<-b.stop

	b.running = false

	return nil
}

// from/for interface Producer
// TODO: send all batches in parallel, will require broader refactoring
func (b *batchProducer) Flush(timeout time.Duration, sendStats bool) (int, int, error) {
	b.Stop()

	timer := time.NewTimer(timeout)
	if timeout == 0 {
		timer.Stop()
	}

	timedOut := false
	sent := 0

loop:
	for len(b.records) > 0 {
		select {
		case <-timer.C:
			timedOut = true
			break loop
		default:
			sent += b.sendBatch(MaxKinesisBatchSize)
		}
	}

	if !timedOut && sendStats {
		b.sendStats()
	}

	return sent, len(b.records), nil
}

func (b *batchProducer) isRunning() bool {
	b.runningMu.RLock()
	defer b.runningMu.RUnlock()
	return b.running
}

// Sends batches of records to Kinesis, possibly re-enqueing them if there are any errors or failed
// records. Returns the number of records successfully sent, if any.
func (b *batchProducer) sendBatch(batchSize int) int {
	if len(b.records) == 0 {
		return 0
	}

	// In the future, maybe this could be a RetryPolicy or something
	if b.consecutiveErrors == 1 {
		b.currentDelay = 50 * time.Millisecond
	} else if b.consecutiveErrors > 1 {
		b.currentDelay *= 2
	}

	if b.currentDelay > 0 {
		b.logger.Printf("Delaying the batch by %v because of %v consecutive errors", b.currentDelay, b.consecutiveErrors)
		time.Sleep(b.currentDelay)
	}

	records := b.takeRecordsFromBuffer(batchSize)
	res, err := b.client.PutRecords(b.recordsToArgs(records))

	if err != nil {
		b.consecutiveErrors++
		b.currentStat.KinesisErrorsSinceLastStat++
		b.logger.Printf("Error occurred when sending PutRecords request to Kinesis stream %v: %v", b.streamName, err)

		if b.consecutiveErrors >= 5 && b.isBufferFullOrNearlyFull() {
			// In order to prevent Add from hanging indefinitely, we start dropping records
			b.logger.Printf("DROPPING %v records because buffer is full or nearly full and there have been %v consecutive errors from Kinesis", len(records), b.consecutiveErrors)
		} else {
			b.logger.Printf("Returning %v records to buffer (%v consecutive errors)", len(records), b.consecutiveErrors)
			// returnRecordsToBuffer can block if the buffer (channel) if full so we’ll
			// call it in a goroutine. This might be problematic WRT ordering. TODO: revisit this.
			go b.returnRecordsToBuffer(records)
		}

		return 0
	}

	b.consecutiveErrors = 0
	b.currentDelay = 0
	succeeded := len(records) - res.FailedRecordCount

	b.currentStat.RecordsSentSuccessfullySinceLastStat += succeeded

	if res.FailedRecordCount == 0 {
		b.logger.Printf("PutRecords request succeeded: sent %v records to Kinesis stream %v", succeeded, b.streamName)
	} else {
		b.logger.Printf("Partial success when sending a PutRecords request to Kinesis stream %v: %v succeeded, %v failed. Re-enqueueing failed records.", b.streamName, succeeded, res.FailedRecordCount)
		// returnSomeFailedRecordsToBuffer can block if the buffer (channel) if full so we’ll
		// call it in a goroutine. This might be problematic WRT ordering. TODO: revisit this.
		go b.returnSomeFailedRecordsToBuffer(res, records)
	}

	return succeeded
}

func (b *batchProducer) isBufferFullOrNearlyFull() bool {
	return float32(len(b.records))/float32(cap(b.records)) >= 0.95
}

func (b *batchProducer) isBufferFull() bool {
	// Treating 99% as full because IIRC, len(chan) has a margin of error
	return float32(len(b.records))/float32(cap(b.records)) >= 0.99
}

func (b *batchProducer) takeRecordsFromBuffer(batchSize int) []batchRecord {
	var size int
	bufferLen := len(b.records)
	if bufferLen >= batchSize {
		size = batchSize
	} else {
		size = bufferLen
	}

	result := make([]batchRecord, size)
	for i := 0; i < size; i++ {
		result[i] = <-b.records
	}
	return result
}

func (b *batchProducer) recordsToArgs(records []batchRecord) *kinesis.RequestArgs {
	args := kinesis.NewArgs()
	args.Add("StreamName", b.streamName)
	for _, record := range records {
		args.AddRecord(record.data, record.partitionKey)
	}
	return args
}

// returnRecordsToBuffer can block if the buffer (channel) is full, so you might want to
// call it in a goroutine.
// TODO: we should probably use a deque internally as the buffer so we can return records to
// the front of the queue, so as to preserve order, which is important.
func (b *batchProducer) returnRecordsToBuffer(records []batchRecord) {
	for _, record := range records {
		// Not using b.Add because we want to preserve the value of record.sendAttempts.
		b.records <- record
	}
}

// returnSomeFailedRecordsToBuffer can block if the buffer (channel) is full, so you might want to
// call it in a goroutine.
// TODO: we should probably use a deque internally as the buffer so we can return records to
// the front of the queue, so as to preserve order, which is important.
func (b *batchProducer) returnSomeFailedRecordsToBuffer(res *kinesis.PutRecordsResp, records []batchRecord) {
	for i, result := range res.Records {
		record := records[i]
		if result.ErrorCode != "" {
			record.sendAttempts++

			if record.sendAttempts < b.config.MaxAttemptsPerRecord {
				b.logger.Printf("Re-enqueueing failed record to buffer for retry. Error code was: '%v' and message was '%v'", result.ErrorCode, result.ErrorMessage)
				// Not using b.Add because we want to preserve the value of record.sendAttempts.
				b.records <- record
			} else {
				b.currentStat.RecordsDroppedSinceLastStat++
				msg := "Dropping failed record; it has hit %v attempts " +
					"which is the maximum. Error code was: '%v' and message was '%v'."
				b.logger.Printf(msg, record.sendAttempts, result.ErrorCode, result.ErrorMessage)
			}
		}
	}
}

func (b *batchProducer) sendStats() {
	if b.config.StatReceiver == nil {
		return
	}

	b.currentStat.BufferSize = len(b.records)

	// I considered running this as a goroutine, but I’m concerned about leaks. So instead, for now,
	// the provider of the BatchStatReceiver must ensure that it is either very fast or non-blocking.
	b.config.StatReceiver.Receive(*b.currentStat)

	b.currentStat = new(StatsBatch)
}
