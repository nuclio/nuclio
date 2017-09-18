package nuclio

import (
	"net/http"
	"testing"
)

func TestNew(t *testing.T) {
	message := "the world is square"
	err := NewErrConflict(message)
	if err.Error() != message {
		t.Fatalf("Bad message: %q != %q", message, err.Error())
	}

	wst, ok := err.(WithStatusCode)
	if !ok {
		t.Fatalf("Not a WithStatusCode error")
	}

	if wst.StatusCode() != http.StatusConflict {
		t.Fatalf("Bad status: %d != %d", wst.StatusCode(), http.StatusConflict)
	}
}
