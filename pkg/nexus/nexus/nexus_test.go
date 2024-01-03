package nexus

import (
	"net/http"
	"testing"

	common "github.com/nuclio/nuclio/pkg/nexus/common/models/structs"
	"github.com/stretchr/testify/suite"
)

type NexusSuite struct {
	suite.Suite
	mockNexus *Nexus
}

func (suite *NexusSuite) SetupTest() {
	suite.mockNexus = Initialize()
}

func (suite *NexusSuite) TestPush() {
	// Push a NexusItem
	mockRequest, _ := http.NewRequest("GET", "http://example.com", nil)

	nexusItem := &common.NexusItem{
		Request: mockRequest,
	}

	suite.mockNexus.Push(nexusItem)

	// Assert that the pushed item is the same as the popped item
	suite.Equal(suite.mockNexus.Queue.Peek(), nexusItem)
	suite.Equal(suite.mockNexus.Queue.Len(), 1)
}

func TestNexusSuite(t *testing.T) {
	suite.Run(t, new(NexusSuite))
}
