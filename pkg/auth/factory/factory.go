/*
Copyright 2023 The Nuclio Authors.

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

package factory

import (
	"github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/auth/iguazio"
	"github.com/nuclio/nuclio/pkg/auth/nop"

	"github.com/nuclio/logger"
)

func NewAuth(logger logger.Logger, authConfig *auth.Config) auth.Auth {
	switch authConfig.Kind {
	case auth.KindIguazio:
		return iguazio.NewAuth(logger, authConfig)
	case auth.KindNop:
		return nop.NewAuth(logger, authConfig)
	default:
		return nop.NewAuth(logger, authConfig)
	}
}
