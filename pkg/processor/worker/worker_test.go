package worker

import (
	"testing"

	"github.com/nuclio/nuclio/pkg/zap"

	"github.com/nuclio/nuclio-sdk"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type MockRuntime struct {
	mock.Mock
}

func (mr *MockRuntime) ProcessEvent(event nuclio.Event) (interface{}, error) {
	args := mr.Called(event)
	return args.Get(0), args.Error(1)
}

type WorkerTestSuite struct {
	suite.Suite
	logger nuclio.Logger
}

func (suite *WorkerTestSuite) SetupSuite() {
	suite.logger, _ = nucliozap.NewNuclioZap("test", nucliozap.DebugLevel)
}

func (suite *WorkerTestSuite) TestProcessEvent() {
	mockRuntime := MockRuntime{}
	worker := NewWorker(suite.logger, 100, &mockRuntime)
	event := &nuclio.AbstractEvent{}

	// expect the mock process event to be called with the event
	mockRuntime.On("ProcessEvent", event).Return(nil, nil).Once()

	// process the event
	worker.ProcessEvent(event)

	// make sure all expectations are met
	mockRuntime.AssertExpectations(suite.T())

	// make sure id was set
	suite.NotNil(event.GetID())
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestWorkerTestSuite(t *testing.T) {
	suite.Run(t, new(WorkerTestSuite))
}
