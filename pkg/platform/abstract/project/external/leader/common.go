/*
Copyright 2026 The Nuclio Authors.

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

package leader

import (
	"fmt"
	"net/http"

	"github.com/nuclio/nuclio-sdk-go"
)

// RequireCASMatch enforces the compare-and-swap invariant shared by every CAS-style leader
// mutation: the caller's prevOpID must match the op_id currently stored on the CRD. An empty
// storedOpID means no CAS key has been written yet — the one-shot migration path for CRDs
// that pre-date 2PC, where nothing has stamped an op_id, so there is nothing to CAS against;
// the request is accepted unconditionally and the current write is what stamps it. After that
// first write, normal CAS enforcement resumes for every subsequent operation.
func RequireCASMatch(storedOpID, prevOpID string) error {
	if storedOpID == "" || storedOpID == prevOpID {
		return nil
	}
	return nuclio.GetByStatusCode(http.StatusConflict)(
		fmt.Sprintf("op_id mismatch (requested %q, stored %q)", prevOpID, storedOpID))
}

// IsOpIDOrdered returns true when newOpID is strictly newer than storedOpID. UUIDv7 encodes a
// millisecond-precision timestamp in the most-significant bits, so lexicographic string
// comparison is equivalent to chronological ordering.
func IsOpIDOrdered(newOpID, storedOpID string) bool {
	return newOpID > storedOpID
}
