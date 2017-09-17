package python

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type IDGeneratorTestSuite struct {
	suite.Suite
}

func (suite *IDGeneratorTestSuite) TestNextID() {
	// We use a map to make sure we didn't get the same ID twice
	ids := make(map[string]bool)
	var lock sync.Mutex
	numRuns := 17
	numGoroutines := 9
	idg := NewIDGenerator()
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := 0; r < numRuns; r++ {
				id := idg.NextID()
				lock.Lock()
				ids[id] = true
				lock.Unlock()
				time.Sleep(time.Duration(821) * time.Nanosecond)
			}
		}()
	}
	wg.Wait()

	suite.Require().Equal(numRuns*numGoroutines, len(ids))
}

func TestIDGenerator(t *testing.T) {
	suite.Run(t, new(IDGeneratorTestSuite))
}
