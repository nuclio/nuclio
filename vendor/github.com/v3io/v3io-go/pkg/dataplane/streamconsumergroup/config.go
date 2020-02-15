package streamconsumergroup

import (
	"time"

	"github.com/v3io/v3io-go/pkg/common"
	"github.com/v3io/v3io-go/pkg/dataplane"
)

type Config struct {
	Session struct {
		Timeout           time.Duration `json:"timeout,omitempty"`
		HeartbeatInterval time.Duration
	} `json:"session,omitempty"`
	State struct {
		ModifyRetry struct {
			Attempts int            `json:"attempts,omitempty"`
			Backoff  common.Backoff `json:"backoff,omitempty"`
		} `json:"modifyRetry,omitempty"`
	} `json:"state,omitempty"`
	SequenceNumber struct {
		CommitInterval    time.Duration `json:"commitInterval,omitempty"`
		ShardWaitInterval time.Duration `json:"shardWaitInterval,omitempty"`
	}
	Claim struct {
		RecordBatchChanSize int `json:"recordBatchChanSize,omitempty"`
		RecordBatchFetch    struct {
			Interval          time.Duration           `json:"interval,omitempty"`
			NumRecordsInBatch int                     `json:"numRecordsInBatch,omitempty"`
			InitialLocation   v3io.SeekShardInputType `json:"initialLocation,omitempty"`
		} `json:"recordBatchFetch,omitempty"`
	} `json:"claim,omitempty"`
}

// NewConfig returns a new configuration instance with sane defaults.
func NewConfig() *Config {
	c := &Config{}
	c.Session.Timeout = 10 * time.Second
	c.Session.HeartbeatInterval = 3 * time.Second
	c.State.ModifyRetry.Attempts = 100
	c.State.ModifyRetry.Backoff = common.Backoff{
		Min:    50 * time.Millisecond,
		Max:    1 * time.Second,
		Factor: 4,
	}
	c.SequenceNumber.CommitInterval = 10 * time.Second
	c.SequenceNumber.ShardWaitInterval = 1 * time.Second
	c.Claim.RecordBatchChanSize = 100
	c.Claim.RecordBatchFetch.Interval = 250 * time.Millisecond
	c.Claim.RecordBatchFetch.NumRecordsInBatch = 10
	c.Claim.RecordBatchFetch.InitialLocation = v3io.SeekShardInputTypeEarliest

	return c
}
