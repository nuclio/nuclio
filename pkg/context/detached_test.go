//go:build test_unit

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
