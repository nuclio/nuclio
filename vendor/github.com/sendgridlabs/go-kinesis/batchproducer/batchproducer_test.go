package batchproducer

import (
	"bytes"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sendgridlabs/go-kinesis"
)

var (
	discardLogger = log.New(ioutil.Discard, "", 0)
	stdoutLogger  = log.New(os.Stdout, "", 0)
)

func TestNewBatchProducerWithGoodValues(t *testing.T) {
	t.Parallel()
	config := Config{
		BufferSize:    10,
		FlushInterval: 0,
		BatchSize:     10,
	}
	b, err := New(&mockBatchingClient{}, "foo", config)
	if b == nil {
		t.Error("b == nil")
	}
	if err != nil {
		t.Errorf("%q != nil", err)
	}
}

func TestNewBatchProducerWithBadBatchSize(t *testing.T) {
	t.Parallel()
	config := Config{
		BufferSize:    10000,
		FlushInterval: 0,
		BatchSize:     1000,
	}
	b, err := New(&mockBatchingClient{}, "foo", config)
	if b != nil {
		t.Errorf("%q != nil", b)
	}
	if err == nil {
		t.Error("err == nil")
	}
	if !strings.Contains(err.Error(), "between 1 and 500") {
		t.Errorf("%q does not contain 'between 1 and 500'", err)
	}
}

func TestNewBatchProducerWithBadValues(t *testing.T) {
	t.Parallel()
	config := Config{
		BufferSize:    10,
		FlushInterval: 0,
		BatchSize:     500,
	}
	b, err := New(&mockBatchingClient{}, "foo", config)
	if b != nil {
		t.Errorf("%q != nil", b)
	}
	if err == nil {
		t.Fatalf("err == nil")
	}
	if !strings.Contains(err.Error(), "Add will block forever") {
		t.Errorf("%q does not contain 'Add will block forever'", err)
	}
}

func TestStart(t *testing.T) {
	t.Parallel()

	b := newProducer(&mockBatchingClient{}, 10, 0, 10)

	if b.isRunning() {
		t.Error("b should not be running")
	}

	err := b.Start()
	defer b.Stop()

	if err != nil {
		t.Errorf("%v != nil", err)
	}

	if !b.isRunning() {
		t.Error("b should be running")
	}
}

func TestStop(t *testing.T) {
	t.Parallel()

	b := newProducer(&mockBatchingClient{}, 10, 0, 10)

	if b.isRunning() {
		t.Error("b should not be running")
	}

	b.Start()
	err := b.Stop()

	if err != nil {
		t.Errorf("%v != nil", err)
	}

	if b.isRunning() {
		t.Error("b should NOT be running")
	}
}

func TestStartWhenStarted(t *testing.T) {
	t.Parallel()
	config := Config{
		BufferSize:    100,
		FlushInterval: 0,
		BatchSize:     10,
	}
	b, err := New(&mockBatchingClient{}, "foo", config)
	if err != nil {
		t.Fatalf("%v != nil", err)
	}

	b.Start()
	defer b.Stop()

	err = b.Start()
	if err == nil {
		t.Errorf("%v == nil", err)
	}
}

func TestStopWhenStopped(t *testing.T) {
	t.Parallel()
	config := Config{
		BufferSize:    100,
		FlushInterval: 0,
		BatchSize:     10,
	}
	b, err := New(&mockBatchingClient{}, "foo", config)
	if err != nil {
		t.Fatalf("%v != nil", err)
	}

	err = b.Stop()
	if err == nil {
		t.Errorf("%v == nil", err)
	}
}

func TestSuccessiveStartsAndStops(t *testing.T) {
	t.Parallel()
	config := Config{
		BufferSize:    100,
		FlushInterval: 0,
		BatchSize:     10,
	}
	b, err := New(&mockBatchingClient{}, "foo", config)
	if err != nil {
		t.Fatalf("%v != nil", err)
	}

	for i := 0; i < 10; i++ {
		err = b.Start()
		if err != nil {
			t.Errorf("%v != nil", err)
		}

		err = b.Stop()
		if err != nil {
			t.Errorf("%v != nil", err)
		}
	}
}

func TestAddRecordWhenStarted(t *testing.T) {
	t.Parallel()
	config := Config{
		BufferSize:    100,
		FlushInterval: 0,
		BatchSize:     10,
	}
	b, err := New(&mockBatchingClient{}, "foo", config)
	if err != nil {
		t.Fatalf("%v != nil", err)
	}

	b.Start()
	defer b.Stop()

	err = b.Add([]byte("foo"), "bar")
	if err != nil {
		t.Errorf("%v != nil", err)
	}
}

func TestAddRecordWhenStopped(t *testing.T) {
	t.Parallel()
	config := Config{
		BufferSize:    100,
		FlushInterval: 0,
		BatchSize:     10,
	}
	b, err := New(&mockBatchingClient{}, "foo", config)
	if err != nil {
		t.Fatalf("%v != nil", err)
	}

	err = b.Add([]byte("foo"), "bar")
	if err == nil {
		t.Errorf("%v == nil", err)
	}
}

func TestFlushInterval(t *testing.T) {
	t.Parallel()
	c := &mockBatchingClient{}
	b := newProducer(c, 100, 2*time.Millisecond, 10)
	b.Start()
	defer b.Stop()

	b.addRecordsAndWait(10, 0)
	if len(b.records) != 10 {
		t.Errorf("%v != 10", len(b.records))
	}
	if c.calls != 0 {
		t.Errorf("%v != 0", c.calls)
	}

	time.Sleep(3 * time.Millisecond)
	if len(b.records) != 0 {
		t.Errorf("%v != 0", len(b.records))
	}
	if c.calls != 1 {
		t.Errorf("%v != 1", c.calls)
	}

	// 20 more records should result in two more batches being sent
	b.addRecordsAndWait(20, 8)
	if len(b.records) != 0 {
		t.Errorf("%v != 0", len(b.records))
	}
	if c.calls != 3 {
		t.Errorf("%v != 3", c.calls)
	}
}

func TestBatchSize(t *testing.T) {
	t.Parallel()
	c := &mockBatchingClient{}
	b := newProducer(c, 100, 0, 5)
	b.Start()
	defer b.Stop()

	b.addRecordsAndWait(4, 2)
	if len(b.records) != 4 {
		t.Errorf("%v != 4", len(b.records))
	}
	if c.calls != 0 {
		t.Errorf("%v != 0", c.calls)
	}

	b.addRecordsAndWait(1, 2)
	if len(b.records) != 0 {
		t.Errorf("%v != 0", len(b.records))
	}
	if c.calls != 1 {
		t.Errorf("%v != 1", c.calls)
	}

	b.addRecordsAndWait(6, 2)
	if len(b.records) != 1 {
		t.Errorf("%v != 1", len(b.records))
	}
	if c.calls != 2 {
		t.Errorf("%v != 2", c.calls)
	}

	b.addRecordsAndWait(19, 2)
	if len(b.records) != 0 {
		t.Errorf("%v != 0", len(b.records))
	}
	if c.calls != 6 {
		t.Errorf("%v != 6", c.calls)
	}
}

func TestBatchError(t *testing.T) {
	t.Parallel()
	c := &mockBatchingClient{shouldErr: true}
	b := newProducer(c, 100, 0, 5)
	b.Start()
	defer b.Stop()

	b.addRecordsAndWait(5, 5)
	if b.consecutiveErrors != 1 {
		t.Errorf("%v != 1", b.consecutiveErrors)
	}
	if len(b.records) != 5 {
		t.Errorf("%v != 5", len(b.records))
	}

	// Wait another 55 ms and another error should have occurred
	time.Sleep(55 * time.Millisecond)
	if b.consecutiveErrors != 2 {
		t.Errorf("%v != 2", b.consecutiveErrors)
	}
	if len(b.records) != 5 {
		t.Errorf("%v != 5", len(b.records))
	}

	b.Stop()
	b.client = &mockBatchingClient{shouldErr: false}
	b.Start()

	time.Sleep(205 * time.Millisecond)
	if b.consecutiveErrors != 0 {
		t.Errorf("%v != 0", b.consecutiveErrors)
	}
	if len(b.records) != 0 {
		t.Errorf("%v != 0", len(b.records))
	}

	// This next batch should succeed immediately
	b.addRecordsAndWait(5, 1)
	if b.consecutiveErrors != 0 {
		t.Errorf("%v != 0", b.consecutiveErrors)
	}
	if len(b.records) != 0 {
		t.Errorf("%v != 0", len(b.records))
	}
}

func TestBatchPartialFailure(t *testing.T) {
	t.Parallel()
	b := newProducer(&mockBatchingClient{}, 100, 0, 20)
	b.config.MaxAttemptsPerRecord = 2
	b.Start()
	defer b.Stop()

	b.addRecordsAndWait(19, 0)

	// Add a single record that will fail. partitionKey is (mis)used to specify that the record
	// should fail.
	b.Add([]byte("foo"), "fail")

	// First attempt
	time.Sleep(5 * time.Millisecond)
	if len(b.records) != 1 {
		t.Errorf("%v != 1", len(b.records))
	}

	// Second attempt
	b.addRecordsAndWait(19, 1)
	// The failing record should be thrown away at this point
	if len(b.records) != 0 {
		t.Errorf("%v != 0", len(b.records))
	}
}

func TestBufferSizeStat(t *testing.T) {
	t.Parallel()

	sr := &statReceiver{}

	b := newProducer(&mockBatchingClient{}, 100, 0, 20)
	b.config.StatReceiver = sr
	b.config.StatInterval = 1 * time.Millisecond
	b.Start()
	defer b.Stop()

	// Adding 10 will not trigger a batch
	b.addRecordsAndWait(10, 2)

	if len(sr.stats) == 0 {
		// More than one might have been sent, which is fine. We just need at least one.
		t.Fatalf("%v == 0", len(sr.stats))
	}

	lastStat := sr.stats[len(sr.stats)-1]
	if lastStat.BufferSize != 10 {
		t.Errorf("%v != 10", lastStat.BufferSize)
	}

	// Adding another 10 **will** trigger a batch
	b.addRecordsAndWait(10, 2)

	if len(sr.stats) < 2 {
		t.Fatalf("%v < 2", len(sr.stats))
	}

	lastStat = sr.stats[len(sr.stats)-1]
	if lastStat.BufferSize != 0 {
		t.Errorf("%v != 0", lastStat.BufferSize)
	}
}

func TestSuccessfulRecordsStat(t *testing.T) {
	t.Parallel()

	sr := &statReceiver{}
	b := newProducer(&mockBatchingClient{}, 100, 0, 20)
	b.config.StatReceiver = sr
	b.config.StatInterval = 1 * time.Millisecond
	// b.logger = stdoutLogger // TEMP TEMP TEMP
	b.Start()
	defer b.Stop()

	// Adding 10 will not trigger a batch
	b.addRecordsAndWait(10, 2)

	if len(sr.stats) == 0 {
		// More than one might have been sent, which is fine. We just need at least one.
		t.Fatalf("%v == 0", len(sr.stats))
	}

	lastStat := sr.stats[len(sr.stats)-1]
	if lastStat.RecordsSentSuccessfullySinceLastStat != 0 {
		t.Errorf("%v != 0", lastStat.RecordsSentSuccessfullySinceLastStat)
	}

	// Adding another 10 **will** trigger a batch
	b.addRecordsAndWait(10, 2)

	if len(sr.stats) < 2 {
		t.Fatalf("%v < 2", len(sr.stats))
	}

	if sr.totalRecordsSentSuccessfully != 20 {
		t.Errorf("%v != 20", sr.totalRecordsSentSuccessfully)
	}
}

func TestSuccessfulRecordsStatWhenSomeRecordsFail(t *testing.T) {
	t.Parallel()

	sr := &statReceiver{}
	b := newProducer(&mockBatchingClient{}, 100, 0, 20)
	b.config.StatReceiver = sr
	b.config.StatInterval = 1 * time.Millisecond
	b.config.MaxAttemptsPerRecord = 2
	b.Start()
	defer b.Stop()

	b.addRecordsAndWait(19, 0)

	// Add a single record that will fail. partitionKey is (mis)used to specify that the record
	// should fail.
	b.Add([]byte("foo"), "fail")

	// Sleep long enough for multiple attempts to be tried
	time.Sleep(3 * time.Millisecond)

	// Should be 10 because one record failed
	if sr.totalRecordsSentSuccessfully != 19 {
		t.Errorf("%v != 19", sr.totalRecordsSentSuccessfully)
	}
}

func TestRecordsDroppedStatWhenSomeRecordsFail(t *testing.T) {
	t.Parallel()

	sr := &statReceiver{}
	b := newProducer(&mockBatchingClient{}, 100, 0, 20)
	b.config.StatReceiver = sr
	b.config.StatInterval = 1 * time.Millisecond
	b.config.MaxAttemptsPerRecord = 1
	b.Start()
	defer b.Stop()

	b.addRecordsAndWait(18, 0)

	// Add two records that will fail. partitionKey is (mis)used to specify that the record
	// should fail.
	b.Add([]byte("foo"), "fail")
	b.Add([]byte("foo"), "fail")

	// Sleep long enough for an attempt to be tried and the stat to be recieved
	time.Sleep(5 * time.Millisecond)

	if sr.totalRecordsDroppedSinceLastStat != 2 {
		t.Errorf("%v != 2", sr.totalRecordsDroppedSinceLastStat)
	}
}

func TestSuccessfulRecordsStatWhenKinesisReturnsError(t *testing.T) {
	t.Parallel()

	sr := &statReceiver{}
	b := newProducer(&mockBatchingClient{shouldErr: true}, 100, 0, 20)
	b.config.StatReceiver = sr
	b.config.StatInterval = 1 * time.Millisecond
	b.Start()
	defer b.Stop()

	// Adding 20 **will** trigger a batch
	b.addRecordsAndWait(20, 50)

	if len(sr.stats) < 1 {
		t.Fatalf("%v < 1", len(sr.stats))
	}

	// Should be 0 because Kinesis is just returning errors
	if sr.totalRecordsSentSuccessfully != 0 {
		t.Errorf("%v != 0", sr.totalRecordsSentSuccessfully)
	}
}

func TestKinesisErrorsStatWhenKinesisSucceeds(t *testing.T) {
	t.Parallel()

	sr := &statReceiver{}
	b := newProducer(&mockBatchingClient{shouldErr: false}, 100, 0, 20)
	b.config.StatReceiver = sr
	b.config.StatInterval = 1 * time.Millisecond
	b.Start()
	defer b.Stop()

	// Adding 20 **will** trigger a batch
	b.addRecordsAndWait(20, 2)

	if len(sr.stats) < 1 {
		t.Fatalf("%v < 1", len(sr.stats))
	}

	// Should be 0 because Kinesis is succeeding
	if sr.totalKinesisErrorsSinceLastStat != 0 {
		t.Errorf("%v != 0", sr.totalKinesisErrorsSinceLastStat)
	}
}

func TestKinesisErrorsStatWhenKinesisReturnsError(t *testing.T) {
	t.Parallel()

	sr := &statReceiver{}
	b := newProducer(&mockBatchingClient{shouldErr: true}, 100, 0, 20)
	b.config.StatReceiver = sr
	b.config.StatInterval = 1 * time.Millisecond
	b.Start()
	defer b.Stop()

	b.addRecordsAndWait(20, 5)
	b.Stop()

	if sr.totalKinesisErrorsSinceLastStat != 2 {
		t.Errorf("%v != 2", sr.totalKinesisErrorsSinceLastStat)
	}
}

func TestLogMessageWhenKinesisSucceeds(t *testing.T) {
	t.Parallel()

	b := newProducer(&mockBatchingClient{shouldErr: false}, 100, 0, 20)
	loggerBuffer, logger := newBufferedLogger()
	b.logger = logger
	b.Start()
	defer b.Stop()

	// Adding 20 **will** trigger a batch
	b.addRecordsAndWait(20, 2)

	loggerString := loggerBuffer.String()
	requiredString := "PutRecords request succeeded: sent 20 records to Kinesis stream"
	if !strings.Contains(loggerString, requiredString) {
		t.Errorf("%s does not contain %s", loggerString, requiredString)
	}
}

func TestLogMessageWhenKinesisReturnsError(t *testing.T) {
	t.Parallel()

	b := newProducer(&mockBatchingClient{shouldErr: true}, 100, 0, 20)
	loggerBuffer, logger := newBufferedLogger()
	b.logger = logger
	b.Start()
	defer b.Stop()

	// Adding 20 **will** trigger a batch
	b.addRecordsAndWait(20, 2)

	loggerString := loggerBuffer.String()
	requiredString := "Error occurred when sending PutRecords request"
	if !strings.Contains(loggerString, requiredString) {
		t.Errorf("%s does not contain %s", loggerString, requiredString)
	}
}

func TestLogMessageWhenSomeRecordsFail(t *testing.T) {
	t.Parallel()

	sr := &statReceiver{}
	b := newProducer(&mockBatchingClient{}, 100, 2*time.Millisecond, 20)
	b.config.StatReceiver = sr
	b.config.StatInterval = 1 * time.Millisecond
	b.config.MaxAttemptsPerRecord = 2
	loggerBuffer, logger := newBufferedLogger()
	b.logger = logger
	b.Start()
	defer b.Stop()

	b.addRecordsAndWait(18, 0)

	// Add two records that will fail. partitionKey is (mis)used to specify that the record
	// should fail.
	b.Add([]byte("foo"), "fail")
	b.Add([]byte("foo"), "fail")

	// Sleep long enough for a few attempts to be tried and the failing records to be re-enqueued
	// and then dropped
	time.Sleep(5 * time.Millisecond)

	loggerString := loggerBuffer.String()

	requiredString := "Partial success when sending a PutRecords request"
	if !strings.Contains(loggerString, requiredString) {
		t.Errorf("%s does not contain %s", loggerString, requiredString)
	}

	requiredString = "Re-enqueueing failed record to buffer for retry. Error code was: 'foo'"
	if !strings.Contains(loggerString, requiredString) {
		t.Errorf("%s does not contain %s", loggerString, requiredString)
	}

	requiredString = "Dropping failed record; it has hit 2 attempts which is the maximum"
	if !strings.Contains(loggerString, requiredString) {
		t.Errorf("%s does not contain %s", loggerString, requiredString)
	}
}

func TestAddBlocksFalse(t *testing.T) {
	t.Parallel()

	b := newProducer(&mockBatchingClient{}, 10, 0, 20)
	b.Start()
	defer b.Stop()

	// Adding 10 will fill up the buffer and not trigger a batch
	b.addRecordsAndWait(10, 2)

	data := []byte("The cheese is old and moldy, where is the bathroom?")
	partitionKey := "foo"
	err := b.Add(data, partitionKey)

	if err == nil {
		t.Errorf("%s == nil", err)
	}
}

func TestAddBlocksTrue(t *testing.T) {
	t.Parallel()

	b := newProducer(&mockBatchingClient{}, 10, 0, 20)
	b.config.AddBlocksWhenBufferFull = true
	b.Start()
	defer b.Stop()

	// Adding 10 will fill up the buffer and not trigger a batch
	b.addRecordsAndWait(10, 2)

	// This should block so we need to run this in a goroutine
	go func() {
		data := []byte("The cheese is old and moldy, where is the bathroom?")
		partitionKey := "foo"
		b.Add(data, partitionKey)
		t.Fatal("We should never have gotten here.")
	}()

	time.Sleep(1 * time.Millisecond)

	if len(b.records) != 10 {
		t.Errorf("%v != 10", len(b.records))
	}
}

func TestFlush(t *testing.T) {
	t.Parallel()

	b := newProducer(&mockBatchingClient{}, 20, 0, 20)
	b.Start()
	defer b.Stop()

	// Adding 10 will not trigger a batch
	b.addRecordsAndWait(10, 2)

	timeout := 20 * time.Second
	sent, remaining, err := b.Flush(timeout, false)
	if err != nil {
		t.Errorf("%s != nil", err)
	}

	if sent != 10 {
		t.Errorf("%v != 10", sent)
	}
	if remaining > 0 {
		t.Errorf("%v > 0", remaining)
	}
	if len(b.records) > 0 {
		t.Errorf("%v > 0", len(b.records))
	}
	if b.isRunning() {
		t.Errorf("b.running != false")
	}
}

func TestFlushWithTimeout(t *testing.T) {
	t.Parallel()

	c := &mockBatchingClient{
		sleepFor: 6 * time.Millisecond,
	}
	b := newProducer(c, 1000, 0, 10)

	// set running to true so Add will succeed
	b.running = true

	// Adding 600 will enqueue 2 batches
	b.addRecordsAndWait(600, 0)

	// back to normal
	b.running = false

	// This should lead to only 1 batch of 500 being sent by Flush
	timeout := 5 * time.Millisecond

	start := time.Now()
	sent, remaining, err := b.Flush(timeout, false)
	duration := time.Since(start)
	if err != nil {
		t.Errorf("%s != nil", err)
	}

	if sent != 500 {
		t.Errorf("%v != 500", sent)
	}
	if remaining != 100 {
		t.Errorf("%v != 100", remaining)
	}
	if len(b.records) != 100 {
		t.Errorf("%v != 100", len(b.records))
	}
	if duration < 6*time.Millisecond || duration > 8*time.Millisecond {
		t.Errorf("%v seems off", duration)
	}
}

func TestFlushWithoutTimeout(t *testing.T) {
	t.Parallel()

	c := &mockBatchingClient{
		sleepFor: 6 * time.Millisecond,
	}
	b := newProducer(c, 1000, 0, 10)

	// set running to true so Add will succeed
	b.running = true

	// Adding 600 will enqueue 2 batches
	b.addRecordsAndWait(600, 0)

	// back to normal
	b.running = false

	// This should lead to batches of 500 and 100 being sent by Flush
	timeout := 0 * time.Millisecond

	start := time.Now()
	sent, remaining, err := b.Flush(timeout, false)
	duration := time.Since(start)
	if err != nil {
		t.Errorf("%s != nil", err)
	}

	if sent != 600 {
		t.Errorf("%v != 600", sent)
	}
	if remaining != 0 {
		t.Errorf("%v != 0", remaining)
	}
	if len(b.records) != 0 {
		t.Errorf("%v != 0", len(b.records))
	}
	if duration < 12*time.Millisecond || duration > 16*time.Millisecond {
		t.Errorf("%v seems off", duration)
	}
}

type mockBatchingClient struct {
	calls     int
	callsMu   sync.Mutex
	shouldErr bool
	numToFail int
	sleepFor  time.Duration
}

func (s *mockBatchingClient) PutRecords(args *kinesis.RequestArgs) (resp *kinesis.PutRecordsResp, err error) {
	s.callsMu.Lock()
	defer s.callsMu.Unlock()
	s.calls++

	if s.shouldErr {
		return nil, errors.New("Oh Noes!")
	}

	time.Sleep(s.sleepFor)

	res := kinesis.PutRecordsResp{Records: make([]kinesis.PutRecordsRespRecord, len(args.Records))}

	for i, record := range args.Records {
		if record.PartitionKey == "fail" {
			res.FailedRecordCount++
			res.Records[i] = kinesis.PutRecordsRespRecord{ErrorCode: "foo", ErrorMessage: "bar"}
		} else {
			res.Records[i] = kinesis.PutRecordsRespRecord{SequenceNumber: "001", ShardId: "001"}
		}
	}

	return &res, nil
}

func newProducer(client *mockBatchingClient, bufferSize int, flushInterval time.Duration, batchSize int) *batchProducer {
	config := Config{
		BufferSize: bufferSize,
		// Set FlushInterval to an interval that will be acceptable to New; we’ll override it below
		// after calling New.
		FlushInterval:        50 * time.Millisecond,
		BatchSize:            batchSize,
		Logger:               discardLogger,
		MaxAttemptsPerRecord: 2,
	}

	producer, err := New(client, "foo", config)
	if err != nil {
		panic(err)
	}

	bp, ok := producer.(*batchProducer)
	if !ok {
		panic("producer is not a *batchProducer!")
	}

	bp.config.FlushInterval = flushInterval

	return bp
}

// There are some cases wherein immediately after adding the records we want to sleep for some
// amount of time in order to allow for the batchProducer’s goroutine to do stuff.
// A possible alternative approach might be to run with multiple CPUs... but that would probably
// still require waiting for at least some small amount of time. And in fact it would be way
// less deterministic and less predictable.
func (b *batchProducer) addRecordsAndWait(numRecords int, millisToWait int) {
	data := []byte("The cheese is old and moldy, where is the bathroom?")
	partitionKey := "foo"
	for i := 0; i < numRecords; i++ {
		err := b.Add(data, partitionKey)
		if err != nil {
			panic(err)
		}
	}

	if millisToWait > 0 {
		time.Sleep(time.Duration(millisToWait) * time.Millisecond)
	}
}

type statReceiver struct {
	stats                            []StatsBatch
	totalKinesisErrorsSinceLastStat  int
	totalRecordsSentSuccessfully     int
	totalRecordsDroppedSinceLastStat int
}

func (s *statReceiver) Receive(sf StatsBatch) {
	s.stats = append(s.stats, sf)
	s.totalKinesisErrorsSinceLastStat += sf.KinesisErrorsSinceLastStat
	s.totalRecordsSentSuccessfully += sf.RecordsSentSuccessfullySinceLastStat
	s.totalRecordsDroppedSinceLastStat += sf.RecordsDroppedSinceLastStat
}

func newBufferedLogger() (*bytes.Buffer, *log.Logger) {
	buf := new(bytes.Buffer)
	logger := log.New(buf, "", 0)
	return buf, logger
}
