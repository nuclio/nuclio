package v3io

import (
	"fmt"
	"testing"

	"github.com/nuclio/nuclio/pkg/zap"
	"github.com/stretchr/testify/suite"
)

//
// Base
//

type ContextTestSuite struct {
	suite.Suite
	logger    Logger
	context   *Context
	session   *Session
	container *Container
}

func (suite *ContextTestSuite) SetupTest() {
	var err error

	suite.logger, err = nucliozap.NewNuclioZapTest("test")

	suite.context, err = NewContext(suite.logger, "192.168.51.240:8081", 8)
	suite.Require().NoError(err, "Failed to create context")

	suite.session, err = suite.context.NewSession("iguazio", "iguazio", "iguazio")
	suite.Require().NoError(err, "Failed to create session")

	suite.container, err = suite.session.NewContainer("1024")
	suite.Require().NoError(err, "Failed to create container")
}

func (suite *ContextTestSuite) TestObject() {
	numRequests := 10
	pathFormat := "object-%d.txt"
	contentsFormat := "contents: %d"

	responseChan := make(chan *Response, 128)

	//
	// Put 10 objects
	//

	// submit 10 put object requests asynchronously
	for requestIndex := 0; requestIndex < numRequests; requestIndex++ {
		request, err := suite.container.PutObject(&PutObjectInput{
			Path: fmt.Sprintf(pathFormat, requestIndex),
			Body: []byte(fmt.Sprintf(contentsFormat, requestIndex)),
		}, responseChan)

		suite.Require().NoError(err)
		suite.Require().NotNil(request)
	}

	// wait on response channel for responses
	suite.waitForResponses(responseChan, numRequests, nil)

	//
	// Get the 10 object's contents
	//

	pendingGetRequests := map[uint64]int{}

	// submit 10 get object requests
	for requestIndex := 0; requestIndex < numRequests; requestIndex++ {
		request, err := suite.container.GetObject(&GetObjectInput{
			Path: fmt.Sprintf(pathFormat, requestIndex),
		}, responseChan)

		suite.Require().NoError(err)
		suite.Require().NotNil(request)

		// store the request ID and the request index so that we can compare
		// the response contents
		pendingGetRequests[request.ID] = requestIndex
	}

	// wait for the responses and verify the contents
	suite.waitForResponses(responseChan, numRequests, func(response *Response) {
		pendingRequestIndex := pendingGetRequests[response.ID]

		// verify that the body of the response is equal to the contents formatting with the request index
		// as mapped from the response ID
		suite.Require().Equal(fmt.Sprintf(contentsFormat, pendingRequestIndex), string(response.Body()))

		// get the request input
		getObjectInput := response.requestResponse.Request.Input.(*GetObjectInput)

		// verify that the request path is correct
		suite.Require().Equal(fmt.Sprintf(pathFormat, pendingRequestIndex), getObjectInput.Path)
	})

	//
	// Delete the objects
	//

	// submit 10 delete object requests asynchronously
	for requestIndex := 0; requestIndex < numRequests; requestIndex++ {
		request, err := suite.container.DeleteObject(&DeleteObjectInput{
			Path: fmt.Sprintf(pathFormat, requestIndex),
		}, responseChan)

		suite.Require().NoError(err)
		suite.Require().NotNil(request)
	}

	// wait on response channel for responses
	suite.waitForResponses(responseChan, numRequests, nil)
}

func (suite *ContextTestSuite) waitForResponses(responseChan chan *Response, numResponses int, verifier func(*Response)) {
	for numResponses != 0 {

		// read a response
		response := <-responseChan

		// verify there's no error
		suite.Require().NoError(response.Error)

		// one less left to wait for
		numResponses--

		if verifier != nil {
			verifier(response)
		}

		// release the response
		response.Release()
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestContextTestSuite(t *testing.T) {
	suite.Run(t, new(ContextTestSuite))
}
