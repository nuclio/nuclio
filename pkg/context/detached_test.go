//go:build test_unit

package context

import (
	"context"
	"testing"
	"time"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type DetachedTestSuite struct {
	suite.Suite
	logger logger.Logger
}

func (suite *DetachedTestSuite) SetupTest() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
}

func (suite *DetachedTestSuite) TestDetachedOnCancelledParent() {
	timerChannel := make(chan int)
	expectedWaitTime := 5

	parentCtx, cancelParentCtx := context.WithCancel(context.Background())
	childCtx, cancelChildCtx := context.WithCancel(NewDetached(parentCtx))

	go suite.measureTimeUntilCancellation(childCtx, timerChannel)

	suite.logger.Debug("Cancelling parent context")
	cancelParentCtx()
	time.Sleep(6 * time.Second)
	suite.logger.Debug("Cancelling child context")
	cancelChildCtx()

	suite.Require().Equal(expectedWaitTime, <-timerChannel)
}

func (suite *DetachedTestSuite) measureTimeUntilCancellation(ctx context.Context, ch chan int) {
	timer := 0
	for {
		select {
		case <-ctx.Done():

			// child context is cancelled
			ch <- timer
			return
		case <-time.After(1 * time.Second):

			// child context is still alive
			timer += 1
		}
	}
}

func TestDetachedTestSuite(t *testing.T) {
	suite.Run(t, new(DetachedTestSuite))
}
