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

package functionres

import (
	"context"

	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"

	"github.com/stretchr/testify/mock"
)

type MockedFunctionRes struct {
	mock.Mock
}

func (mfr *MockedFunctionRes) List(ctx context.Context, s string) ([]Resources, error) {
	args := mfr.Called(ctx, s)
	return args.Get(0).([]Resources), args.Error(1)
}

func (mfr *MockedFunctionRes) Get(ctx context.Context, s string, s2 string) (Resources, error) {
	args := mfr.Called(ctx, s, s2)
	return args.Get(0).(Resources), args.Error(1)
}

func (mfr *MockedFunctionRes) CreateOrUpdate(ctx context.Context, function *nuclioio.NuclioFunction, s string) (Resources, error) {
	args := mfr.Called(ctx, function, s)
	return args.Get(0).(Resources), args.Error(1)
}

func (mfr *MockedFunctionRes) WaitAvailable(ctx context.Context, s string, s2 string) error {
	args := mfr.Called(ctx, s, s2)
	return args.Error(0)
}

func (mfr *MockedFunctionRes) Delete(ctx context.Context, s string, s2 string) error {
	args := mfr.Called(ctx, s, s2)
	return args.Error(0)
}

func (mfr *MockedFunctionRes) SetPlatformConfigurationProvider(provider PlatformConfigurationProvider) {
	mfr.Called(provider)
}
