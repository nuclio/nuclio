package v3io

import (
	"testing"

	"fmt"
	"github.com/nuclio/nuclio/pkg/zap"
	"github.com/stretchr/testify/suite"
	"sync"
)

//
// Base
//

type ClientTestSuite struct {
	suite.Suite
	logger    Logger
	client    *Client
	session   *Session
	container *Container
}

func (suite *ClientTestSuite) SetupTest() {
	var err error

	suite.logger, err = nucliozap.NewNuclioZap("test", nucliozap.DebugLevel)

	suite.client, err = NewClient(suite.logger, "192.168.51.12:8081")
	suite.Require().NoError(err, "Failed to create client")

	suite.session, err = suite.client.NewSession("iguazio", "iguazio", "iguazio")
	suite.Require().NoError(err, "Failed to create session")

	suite.container, err = suite.session.NewContainer("1024")
	suite.Require().NoError(err, "Failed to create container")
}

//
// API tests (all commands and such)
//

type ClientApiTestSuite struct {
	ClientTestSuite
}

//func (suite *ClientApiTestSuite) TestListAll() {
//
//	// get all buckets
//	response, err := suite.container.ListAll()
//	suite.Require().NoError(err, "Failed to list all")
//
//	output := response.output.(*ListAllOutput)
//
//	// make sure buckets is not empty
//	suite.Require().NotEmpty(output.Buckets, "Expected at least one bucket")
//
//	// release the response
//	response.Release()
//}

func (suite *ClientApiTestSuite) TestListBucket() {
	// suite.T().Skip()

	input := ListBucketInput{
		Path: "",
	}

	// get a specific bucket
	response, err := suite.container.ListBucket(&input)
	suite.Require().NoError(err, "Failed to list bucket")

	output := response.output.(*ListBucketOutput)

	// make sure buckets is not empty
	suite.Require().NotEmpty(output.Contents, "Expected at least one item")

	// release the response
	response.Release()
}

func (suite *ClientApiTestSuite) TestObject() {
	// suite.T().Skip()

	path := "object.txt"
	contents := "vegans are better than everyone"

	//
	// PUT contents to some object
	//

	err := suite.container.PutObject(&PutObjectInput{
		Path: path,
		Body: []byte(contents),
	})

	suite.Require().NoError(err, "Failed to put")

	//
	// Get the contents
	//

	response, err := suite.container.GetObject(&GetObjectInput{
		Path: path,
	})

	suite.Require().NoError(err, "Failed to get")

	// make sure buckets is not empty
	suite.Require().Equal(contents, string(response.Body()))

	// release the response
	response.Release()

	//
	// Delete the object
	//

	err = suite.container.DeleteObject(&DeleteObjectInput{
		Path: path,
	})

	suite.Require().NoError(err, "Failed to delete")

	//
	// Get the contents again (should fail)
	//

	response, err = suite.container.GetObject(&GetObjectInput{
		Path: path,
	})

	suite.Require().Error(err, "Failed to get")
	suite.Require().Nil(response)
}

func (suite *ClientApiTestSuite) TestEMD() {
	// suite.T().Skip()

	records := map[string]map[string]interface{}{
		"bob":    {"age": 42, "feature": "mustance"},
		"linda":  {"age": 41, "feature": "singing"},
		"louise": {"age": 9, "feature": "bunny ears"},
		"tina":   {"age": 14, "feature": "butts"},
	}

	// create the records
	for recordKey, recordAttributes := range records {
		input := PutItemInput{
			Path:       "emd0/" + recordKey,
			Attributes: recordAttributes,
		}

		// get a specific bucket
		err := suite.container.PutItem(&input)
		suite.Require().NoError(err, "Failed to put item")
	}

	// update louise record
	updateItemInput := UpdateItemInput{
		Path: "emd0/louise",
		Attributes: map[string]interface{}{
			"height": 130,
			"quip":   "i can smell fear on you",
		},
	}

	err := suite.container.UpdateItem(&updateItemInput)
	suite.Require().NoError(err, "Failed to update item")

	// get tina
	getItemInput := GetItemInput{
		Path:           "emd0/louise",
		AttributeNames: []string{"__size", "age", "quip", "height"},
	}

	response, err := suite.container.GetItem(&getItemInput)
	suite.Require().NoError(err, "Failed to get item")

	getItemOutput := response.output.(*GetItemOutput)

	// make sure we got the age and quip correctly
	suite.Require().Equal(0, getItemOutput.Attributes["__size"].(int))
	suite.Require().Equal(130, getItemOutput.Attributes["height"].(int))
	suite.Require().Equal("i can smell fear on you", getItemOutput.Attributes["quip"].(string))
	suite.Require().Equal(9, getItemOutput.Attributes["age"].(int))

	// release the response
	response.Release()

	//
	// Delete everything
	//

	// delete the records
	for recordKey, _ := range records {
		input := DeleteObjectInput{
			Path: "emd0/" + recordKey,
		}

		// get a specific bucket
		err := suite.container.DeleteObject(&input)
		suite.Require().NoError(err, "Failed to delete item")
	}

	// delete the directory
	err = suite.container.DeleteObject(&DeleteObjectInput{
		Path: "emd0/",
	})

	suite.Require().NoError(err, "Failed to delete")
}

//
// Stress test
//

type ClientStressTestSuite struct {
	ClientTestSuite
}

func (suite *ClientStressTestSuite) TestStressPutGet() {
	pathTemplate := "stress/stress-%d.txt"
	contents := "0123456789"

	waitGroup := sync.WaitGroup{}

	// spawn workers - each putting / getting a different object
	for workerIndex := 0; workerIndex < 32; workerIndex++ {
		waitGroup.Add(1)

		go func(workerIndex int) {
			path := fmt.Sprintf(pathTemplate, workerIndex)

			for iteration := 0; iteration < 100; iteration++ {

				err := suite.container.PutObject(&PutObjectInput{
					Path: path,
					Body: []byte(contents),
				})

				suite.Require().NoError(err, "Failed to put")

				response, err := suite.container.GetObject(&GetObjectInput{
					Path: path,
				})

				suite.Require().NoError(err, "Failed to get")

				// release the response
				response.Release()
			}

			// delete the object
			err := suite.container.DeleteObject(&DeleteObjectInput{
				Path: path,
			})

			suite.Require().NoError(err, "Failed to delete")

			// signal that this worker is done
			waitGroup.Done()
		}(workerIndex)
	}

	waitGroup.Wait()
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ClientApiTestSuite))
	suite.Run(t, new(ClientStressTestSuite))
}
