package test

import (
	"fmt"
	"testing"

	"github.com/v3io/v3io-go/pkg/dataplane"

	"github.com/stretchr/testify/suite"
)

type asyncTestSuite struct {
	testSuite
}

func (suite *asyncTestSuite) waitForResponses(responseChan chan *v3io.Response, numResponses int, verifier func(*v3io.Response)) {
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

//
// Object tests
//

type asyncObjectTestSuite struct {
	asyncTestSuite
}

func (suite *asyncObjectTestSuite) TestObject() {
	numRequests := 10
	pathFormat := "/object-%d.txt"
	contentsFormat := "contents: %d"
	someContext := 30

	responseChan := make(chan *v3io.Response, 128)

	//
	// Put 10 objects
	//

	// submit 10 put object requests asynchronously
	for requestIndex := 0; requestIndex < numRequests; requestIndex++ {
		putObjectInput := &v3io.PutObjectInput{
			Path: fmt.Sprintf(pathFormat, requestIndex),
			Body: []byte(fmt.Sprintf(contentsFormat, requestIndex)),
		}

		// when run against a context, will populate fields like container name
		suite.populateDataPlaneInput(&putObjectInput.DataPlaneInput)

		request, err := suite.container.PutObject(putObjectInput, &someContext, responseChan)

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
		getObjectInput := &v3io.GetObjectInput{
			Path: fmt.Sprintf(pathFormat, requestIndex),
		}

		// when run against a context, will populate fields like container name
		suite.populateDataPlaneInput(&getObjectInput.DataPlaneInput)

		request, err := suite.container.GetObject(getObjectInput, &someContext, responseChan)

		suite.Require().NoError(err)
		suite.Require().NotNil(request)

		// store the request ID and the request index so that we can compare
		// the response contents
		pendingGetRequests[request.ID] = requestIndex
	}

	// wait for the responses and verify the contents
	suite.waitForResponses(responseChan, numRequests, func(response *v3io.Response) {
		pendingRequestIndex := pendingGetRequests[response.ID]

		// verify the context
		suite.Require().Equal(&someContext, response.Context)

		// verify that the body of the response is equal to the contents formatting with the request index
		// as mapped from the response ID
		suite.Require().Equal(fmt.Sprintf(contentsFormat, pendingRequestIndex), string(response.Body()))

		// get the request input
		getObjectInput := response.RequestResponse.Request.Input.(*v3io.GetObjectInput)

		// verify that the request path is correct
		suite.Require().Equal(fmt.Sprintf(pathFormat, pendingRequestIndex), getObjectInput.Path)
	})

	//
	// Delete the objects
	//

	// submit 10 delete object requests asynchronously
	for requestIndex := 0; requestIndex < numRequests; requestIndex++ {
		deleteObjectInput := &v3io.DeleteObjectInput{
			Path: fmt.Sprintf(pathFormat, requestIndex),
		}

		// when run against a context, will populate fields like container name
		suite.populateDataPlaneInput(&deleteObjectInput.DataPlaneInput)

		request, err := suite.container.DeleteObject(deleteObjectInput, &someContext, responseChan)

		suite.Require().NoError(err)
		suite.Require().NotNil(request)
	}

	// wait on response channel for responses
	suite.waitForResponses(responseChan, numRequests, nil)
}

type asyncContextObjectTestSuite struct {
	asyncObjectTestSuite
}

func (suite *asyncContextObjectTestSuite) SetupSuite() {
	suite.asyncObjectTestSuite.SetupSuite()

	suite.createContext()
}

type asyncContainerObjectTestSuite struct {
	asyncObjectTestSuite
}

func (suite *asyncContainerObjectTestSuite) SetupSuite() {
	suite.asyncObjectTestSuite.SetupSuite()

	suite.createContainer()
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestAsyncSuite(t *testing.T) {
	suite.Run(t, new(asyncContextObjectTestSuite))
	suite.Run(t, new(asyncContainerObjectTestSuite))
}
