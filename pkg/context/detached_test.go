//go:build test_unit

package context

import (
	"context"
	"sync"
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

func (suite *DetachedTestSuite) TestCancelParent() {
	waitGroup := sync.WaitGroup{}
	isChildDone := false
	parentCtx, cancelParentCtx := context.WithCancel(context.Background())
	childCtx, cancelChildCtx := context.WithCancel(NewDetached(parentCtx))

	go suite.waitForContextCancellation(childCtx, &isChildDone, &waitGroup)

	cancelParentCtx()

	time.After(2 * time.Second)
	cancelChildCtx()
	waitGroup.Wait()

	suite.Require().True(isChildDone)
}

func (suite *DetachedTestSuite) waitForContextCancellation(ctx context.Context, isCtxDone *bool, waitGroup *sync.WaitGroup) {
	waitGroup.Add(1)
	for {
		select {
		case <-ctx.Done():

			// child context is cancelled
			*isCtxDone = true
			waitGroup.Done()
			return
		case <-time.After(1 * time.Second):

			// child context is still alive
			suite.Require().False(*isCtxDone)
		}
	}
}

func TestDetachedTestSuite(t *testing.T) {
	suite.Run(t, new(DetachedTestSuite))
}
