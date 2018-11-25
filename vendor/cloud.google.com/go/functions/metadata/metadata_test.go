// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metadata

import (
	"context"
	"testing"
)

func TestMetadata(t *testing.T) {
	meta := Metadata{
		EventID: "test event ID",
	}
	ctx := NewContext(context.Background(), meta)
	newMeta, ok := FromContext(ctx)
	if !ok {
		t.Fatalf("No context metadata found")
	}
	if newMeta != meta {
		t.Fatalf("got %v, want %v", newMeta, meta)
	}
}
