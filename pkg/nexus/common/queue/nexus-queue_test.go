package common

import (
	commonNexus "github.com/nuclio/nuclio/pkg/nexus/common/models/structs"
	"github.com/stretchr/testify/suite"
	"net/http"
	"testing"
	"time"
)

type HelperSuite struct {
	suite.Suite
	mockPriorityQueue *NexusQueue
}

func (suite *HelperSuite) SetupTest() {
	suite.mockPriorityQueue = Initialize()
}

func createMockItemList() ([]*commonNexus.NexusItem, *commonNexus.NexusItem) {
	startTime := time.Now()
	mockRequest := &http.Request{}

	mockItemList := []*commonNexus.NexusItem{
		commonNexus.NewNexusItem(mockRequest, startTime, "test1"),
		commonNexus.NewNexusItem(mockRequest, startTime.Add(20*time.Second), "test2"),
	}

	return mockItemList, mockItemList[0]
}

func (suite *HelperSuite) TestPriorityQueue() {
	mockItemList, firstItem := createMockItemList()

	// Test Push
	for _, item := range mockItemList {
		suite.mockPriorityQueue.Push(item)
	}
	suite.Equal(2, suite.mockPriorityQueue.Len())

	// Test Peek
	suite.Equal(firstItem, suite.mockPriorityQueue.Peek())

	// Test Update
	newDeadline1 := suite.mockPriorityQueue.Peek().Deadline.Add(40 * time.Minute)
	suite.mockPriorityQueue.Update(suite.mockPriorityQueue.Peek(), newDeadline1)
	suite.False(suite.mockPriorityQueue.Peek().Deadline.Equal(newDeadline1))

	newDeadline2 := suite.mockPriorityQueue.Peek().Deadline.Add(40 * time.Minute)
	suite.mockPriorityQueue.Update(suite.mockPriorityQueue.Peek(), newDeadline2)

	// Test Pop
	firstItem = suite.mockPriorityQueue.Peek()
	popped := suite.mockPriorityQueue.Pop()
	suite.Equal(firstItem, popped)
	suite.Equal(1, suite.mockPriorityQueue.Len())

	// Test Remove
	suite.mockPriorityQueue.Remove(suite.mockPriorityQueue.Peek())
	suite.Equal(0, suite.mockPriorityQueue.Len())
}

func (suite *HelperSuite) TestDeadlineImpl() {
	mockDeadlineHeap := &nexusHeap{}

	// Test Len
	suite.Equal(0, mockDeadlineHeap.Len())

	// Test Push
	mockItemList, firstItem := createMockItemList()

	// Test Push
	for _, item := range mockItemList {
		mockDeadlineHeap.Push(item)
	}
	suite.Equal(2, mockDeadlineHeap.Len())

	// Test Less
	suite.True(mockDeadlineHeap.Less(0, 1))

	// Test Swap
	mockDeadlineHeap.Swap(0, 1)
	suite.False(mockDeadlineHeap.Less(0, 1))

	// Test Pop
	popped := mockDeadlineHeap.Pop()
	suite.Equal(firstItem, popped)
	suite.Equal(1, mockDeadlineHeap.Len())
}

func TestSuite(t *testing.T) {
	suite.Run(t, new(HelperSuite))
}
