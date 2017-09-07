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

package nucliozap

import (
	"testing"

	"github.com/nuclio/nuclio-sdk"
)

func TestSimpleLogging(t *testing.T) {
	var baseLogger nuclio.Logger

	baseLogger, err := NewNuclioZapTest("test")
	if err != nil {
		t.Error(err)
	}

	baseLogger.Debug("Hello there")
	baseLogger.Info("Hello there info")
	baseLogger.Warn("Hello there %s", "warning")
	baseLogger.DebugWith("Hello there", "a", 3, "b", "foo")
	baseLogger.Error("An error %s %d", "aaa", 30)

	childLogger1 := baseLogger.GetChild("child1").(nuclio.Logger)
	childLogger1.DebugWith("What", "a", 1)

	childLogger2 := childLogger1.GetChild("child2").(nuclio.Logger)
	childLogger2.DebugWith("Foo", "a", 1)
	childLogger2.DebugWith("Foo", "a", 1)
}
