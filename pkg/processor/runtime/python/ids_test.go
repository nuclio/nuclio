/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
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
