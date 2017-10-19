package http

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	suite.Suite
}

func (suite *TestSuite) TestPool() {
	size := 7
	pool := NewFixedEventPool(size)

	suite.Require().Equal(size, cap(pool))

	// Spin some goroutines to get/return events
	numJobs := 3
	numBorrows := 100
	totalBorrows := uint64(0)
	var wg sync.WaitGroup

	for i := 0; i < numJobs; i++ {
		wg.Add(1)
		go func() {
			for i := 0; i < numBorrows; i++ {
				event := pool.Get()
				time.Sleep(time.Nanosecond)
				pool.Put(event)
				atomic.AddUint64(&totalBorrows, 1)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	suite.Require().Equal(totalBorrows, uint64(numJobs*numBorrows))
}

func TestFixedEventPool(t *testing.T) {
	suite.Run(t, new(TestSuite))
}
