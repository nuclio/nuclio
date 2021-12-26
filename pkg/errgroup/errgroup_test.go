// +build test_unit

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

package errgroup

import (
	"context"
	"sync"
	"testing"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type ErrGroupTestSuite struct {
	suite.Suite
	logger logger.Logger
	ctx    context.Context
	lock   sync.Mutex
}

func (suite *ErrGroupTestSuite) SetupTest() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
	suite.ctx = context.Background()
	suite.lock = sync.Mutex{}
}

func (suite *ErrGroupTestSuite) TestSemaphoredErrGroup() {

	for _, testCase := range []struct {
		name          string
		concurrency   int
		goroutinesNum int
	}{
		{
			name:          "DefaultConcurrency",
			concurrency:   DefaultErrgroupConcurrency,
			goroutinesNum: 10,
		},
		{
			name:          "ZeroConcurrency",
			concurrency:   0,
			goroutinesNum: 10,
		},
		{
			name:          "NegativeConcurrency",
			concurrency:   -45,
			goroutinesNum: 10,
		},
		{
			name:          "ManyGoroutines",
			concurrency:   7,
			goroutinesNum: 30,
		},
	} {
		suite.Run(testCase.name, func() {
			var concurrentCallCount, totalCallCount int
			errGroup, errGroupCtx := WithContextSemaphore(suite.ctx, suite.logger, testCase.concurrency)

			for i := 0; i < testCase.goroutinesNum; i++ {
				errGroup.Go(testCase.name, func() error {
					suite.lock.Lock()
					concurrentCallCount++
					totalCallCount++
					suite.lock.Unlock()

					suite.logger.DebugWithCtx(errGroupCtx, "In a goroutine", "callCount", concurrentCallCount)
					if testCase.concurrency <= 0 {
						suite.Require().LessOrEqual(concurrentCallCount, DefaultErrgroupConcurrency)
					} else {
						suite.Require().LessOrEqual(concurrentCallCount, testCase.concurrency)
					}

					suite.lock.Lock()
					concurrentCallCount--
					suite.lock.Unlock()

					return nil
				})
			}

			suite.Require().NoError(errGroup.Wait())
			suite.Require().Equal(concurrentCallCount, 0)
			suite.Require().Equal(totalCallCount, testCase.goroutinesNum)
		})
	}
}

func TestErrGroupTestSuite(t *testing.T) {
	suite.Run(t, new(ErrGroupTestSuite))
}
