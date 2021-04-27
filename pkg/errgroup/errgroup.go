/*
Copyright The Kubernetes Authors.

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
	"runtime/debug"

	"github.com/nuclio/nuclio/pkg/common"

	"github.com/nuclio/logger"
	"golang.org/x/sync/errgroup"
)

type Group struct {
	*errgroup.Group
	logger logger.Logger
	ctx    context.Context
}

func WithContext(ctx context.Context, loggerInstance logger.Logger) (*Group, context.Context) {
	newBaseErrgroup, errgroupCtx := errgroup.WithContext(ctx)

	return &Group{
		Group:  newBaseErrgroup,
		logger: loggerInstance,
		ctx:    errgroupCtx,
	}, errgroupCtx
}

func (g *Group) Go(actionName string, f func() error) {
	wrapper := func() (err error) {
		defer func() {
			if recoveredErr := recover(); recoveredErr != nil {
				callStack := debug.Stack()
				common.LogPanic(g.ctx, g.logger, actionName, nil, callStack, recoveredErr)
				err = common.ErrorFromRecoveredError(recoveredErr)
			}
		}()
		err = f()
		return
	}
	g.Group.Go(wrapper)
}
