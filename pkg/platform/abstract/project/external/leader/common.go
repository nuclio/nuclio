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

	"github.com/nuclio/errors"
	"github.com/nuclio/nuclio-sdk-go"
)

// RequireCASMatch enforces the compare-and-swap invariant shared by every CAS-style leader
// mutation: the caller's prevOpID must match the op_id currently stored on the CRD. An empty
// storedOpID means no CAS key has been written yet — the one-shot migration path for CRDs
// that pre-date 2PC, where nothing has stamped an op_id, so there is nothing to CAS against;
// the request is accepted unconditionally and the current write is what stamps it. After that
// first write, normal CAS enforcement resumes for every subsequent operation.
func RequireCASMatch(prevOpID, storedOpID string) error {
	if storedOpID == "" || IsOpIDEqual(storedOpID, prevOpID) {
		return nil
	}
	return nuclio.GetByStatusCode(http.StatusConflict)(
		fmt.Sprintf("op_id mismatch; requested:(%q), stored:(%q)", prevOpID, storedOpID))
}

// IsOpIDOrdered returns true when newOpID is strictly newer than storedOpID. UUIDv7 encodes a
// millisecond-precision timestamp in the most-significant bits, so lexicographic string
// comparison is equivalent to chronological ordering.
func IsOpIDOrdered(newOpID, storedOpID string) bool {
	return newOpID > storedOpID
}

// IsOpIDEqual returns true when newOpID is equal to storedOpID.
func IsOpIDEqual(newOpID, storedOpID string) bool {
	return newOpID == storedOpID
}

// RequireOpIDMatch returns a simple error when requestedOpID does not equal storedOpID, the
// phase-binding check shared by callers where the request must match the op_id already
// written on the CRD by a preceding phase. The caller wraps the result with its own status
// code and operation-specific message.
func RequireOpIDMatch(requestedOpID, storedOpID string) error {
	if requestedOpID == storedOpID {
		return nil
	}
	return errors.Errorf("op_id mismatch (requested %q, stored %q)", requestedOpID, storedOpID)
}

// RequireOpIDOrdered returns a simple error when newOpID is not strictly newer than storedOpID,
// the replay-protection guard shared by every phase that advances the stored op_id. The caller
// wraps the result with its own status code and operation-specific message.
func RequireOpIDOrdered(newOpID, storedOpID string) error {
	if IsOpIDOrdered(newOpID, storedOpID) {
		return nil
	}
	return errors.Errorf("op_id is not newer than stored op_id (requested %q, stored %q)", newOpID, storedOpID)
}
