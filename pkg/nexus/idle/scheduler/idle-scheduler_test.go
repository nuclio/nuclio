package idle_test

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/nexus/common/models/config"
	"github.com/nuclio/nuclio/pkg/nexus/common/models/structs"
	common "github.com/nuclio/nuclio/pkg/nexus/common/queue"
	scheduler "github.com/nuclio/nuclio/pkg/nexus/common/scheduler"
	idle "github.com/nuclio/nuclio/pkg/nexus/idle/scheduler"
	"github.com/stretchr/testify/suite"
)

type IdleSchedulerTestSuite struct {
	suite.Suite
	is idle.IdleScheduler
}

var mockRequest = &http.Request{
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

func (suite *IdleSchedulerTestSuite) SetupTest() {
	sleepDuration := 10 * time.Millisecond

	defaultQueue := common.
		Initialize()
	baseSchedulerConfig := config.BaseNexusSchedulerConfig{
		SleepDuration: sleepDuration,
	}
	nexusConfig := config.
		NewDefaultNexusConfig()

	Client := &http.Client{
		Transport: &MockRoundTripper{},
	}

	baseScheduler := scheduler.
		NewBaseNexusScheduler(defaultQueue, &baseSchedulerConfig, &nexusConfig, Client)

	suite.is = *idle.NewScheduler(baseScheduler)
}

func (suite *IdleSchedulerTestSuite) pushTasksToQueue(amount int, offset int) {
	for i := 1; i <= amount; i++ {
		task := &structs.NexusItem{
			Name:     fmt.Sprintf("task_%d", i),
			Deadline: time.Now().Add(time.Duration(i*offset) * time.Millisecond),
			Request:  mockRequest,
		}
		suite.is.Push(task)
	}
}

func (suite *IdleSchedulerTestSuite) TestIdleScheduler() {
	suite.pushTasksToQueue(10, 10)

	suite.is.MaxParallelRequests.Store(1)

	go suite.is.Start()

	time.Sleep(5 * time.Millisecond)

	for i := 1; i <= 10; i++ {
		suite.Equal(10-i, suite.is.Queue.Len())
		time.Sleep(10 * time.Millisecond)

	}

}

func TestIdleSchedulerTestSuite(t *testing.T) {
	suite.Run(t, new(IdleSchedulerTestSuite))
}
