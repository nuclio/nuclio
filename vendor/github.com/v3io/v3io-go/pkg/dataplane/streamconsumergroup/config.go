package streamconsumergroup

import (
	"time"

	"github.com/v3io/v3io-go/pkg/common"
	"github.com/v3io/v3io-go/pkg/dataplane"
)

type Config struct {
	Session struct {
		Timeout time.Duration
	}
	State struct {
		ModifyRetry struct {
			Attempts int
			Backoff  common.Backoff
		}
		Heartbeat struct {
			Interval time.Duration
		}
	}
	SequenceNumber struct {
		Commit struct {
			Interval time.Duration
		}
	}
	Claim struct {
		RecordBatchChanSize int
		RecordBatchFetch    struct {
			Interval          time.Duration
			NumRecordsInBatch int
			InitialLocation   v3io.SeekShardInputType
		}
	}
}

// NewConfig returns a new configuration instance with sane defaults.
func NewConfig() *Config {
	c := &Config{}
	c.Session.Timeout = 10 * time.Second
	c.State.ModifyRetry.Attempts = 100
	c.State.ModifyRetry.Backoff = common.Backoff{
		Min:    50 * time.Millisecond,
		Max:    1 * time.Second,
		Factor: 4,
	}
	c.State.Heartbeat.Interval = 3 * time.Second
	c.SequenceNumber.Commit.Interval = 10 * time.Second
	c.Claim.RecordBatchChanSize = 100
	c.Claim.RecordBatchFetch.Interval = 250 * time.Millisecond
	c.Claim.RecordBatchFetch.NumRecordsInBatch = 10
	c.Claim.RecordBatchFetch.InitialLocation = v3io.SeekShardInputTypeEarliest

	return c
}
