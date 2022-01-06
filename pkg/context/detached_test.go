//go:build test_unit

package context

import (
	"context"
	"testing"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type DetachedTestSuite struct {
	suite.Suite
	logger logger.Logger
}

// new type for context key usage
type key int

const ctxKey key = iota

func (suite *DetachedTestSuite) SetupTest() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
}

func (suite *DetachedTestSuite) TestCancelParentWithValue() {
	parentCtx, parentCancel := context.WithCancel(context.WithValue(context.TODO(), ctxKey, "someValue"))
	childCtx, childCancel := context.WithCancel(NewDetached(parentCtx))

	// cancel parent
	parentCancel()

	// parent is canceled
	suite.Require().NotNil(parentCtx.Err())

	// child is not canceled
	suite.Require().Nil(childCtx.Err())

	// child can still access his canceled-parent
	suite.Require().Equal(childCtx.Value(ctxKey), "someValue")

	// cancel child
	childCancel()

	// child is canceled
	suite.Require().NotNil(childCtx.Err())

	// sanity all context are done and canceled
	<-childCtx.Done()
	<-parentCtx.Done()
}

func TestDetachedTestSuite(t *testing.T) {
	suite.Run(t, new(DetachedTestSuite))
}
