package utils_test

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/nuclio/nuclio/pkg/nexus/common/models/structs"
	common "github.com/nuclio/nuclio/pkg/nexus/common/scheduler"
)

var MockRequest = &http.Request{
	Method: "GET",
	URL: &url.URL{
		Path:   "/api",
		Scheme: "http",
		Host:   "localhost:8070",
	},
	Header: make(http.Header),
}

type MockRoundTripper struct{}

func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString("Mocked response")),
		Header:     make(http.Header),
	}, nil
}

func PushMockedTasksToQueue(queue *common.BaseNexusScheduler, names []string, offset int) {
	for i, name := range names {
		task := &structs.NexusItem{
			Name:     name,
			Deadline: time.Now().Add(time.Duration(i*offset) * time.Millisecond),
			Request:  MockRequest,
		}
		queue.Push(task)
	}
}
