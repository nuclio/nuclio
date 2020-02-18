package streamconsumergroup

import (
	"time"

	v3io "github.com/v3io/v3io-go/pkg/dataplane"
)

type stateModifier func(*State) (*State, error)

type SessionState struct {
	MemberID      string    `json:"member_id"`
	LastHeartbeat time.Time `json:"last_heartbeat_time"`
	Shards        []int     `json:"shards"`
}

type Handler interface {

	// Setup is run at the beginning of a new session, before ConsumeClaim.
	Setup(Session) error

	// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
	// but before the locations are committed for the very last time.
	Cleanup(Session) error

	// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages().
	// Once the Messages() channel is closed, the Handler must finish its processing
	// loop and exit.
	ConsumeClaim(Session, Claim) error
}

type RecordBatch struct {
	Records      []v3io.StreamRecord
	Location     string
	NextLocation string
	ShardID      int
}

type StreamConsumerGroup interface {
	GetState() (*State, error)
	GetShardSequenceNumber(int) (uint64, error)
	GetNumShards() (int, error)
}

type Member interface {
	Consume(Handler) error
	Close() error
}

type Session interface {
	GetClaims() []Claim
	GetMemberID() string
	MarkRecord(*v3io.StreamRecord) error

	start() error
	stop() error
}

type Claim interface {
	GetStreamPath() string
	GetShardID() int
	GetCurrentLocation() string
	GetRecordBatchChan() <-chan *RecordBatch

	start() error
	stop() error
}
