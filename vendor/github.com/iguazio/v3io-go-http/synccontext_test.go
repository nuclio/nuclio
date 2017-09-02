package v3io

import (
	"fmt"
	"sync"
	"testing"

	"github.com/nuclio/nuclio/pkg/zap"
	"github.com/stretchr/testify/suite"
)

//
// Base
//

type SyncContextTestSuite struct {
	suite.Suite
	logger    Logger
	context   *Context
	session   *Session
	container *Container
}

func (suite *SyncContextTestSuite) SetupTest() {
	var err error

	suite.logger, err = nucliozap.NewNuclioZap("test", nucliozap.DebugLevel)

	suite.context, err = NewContext(suite.logger, "192.168.51.240:8081", 1)
	suite.Require().NoError(err, "Failed to create context")

	suite.session, err = suite.context.NewSession("iguazio", "iguazio", "iguazio")
	suite.Require().NoError(err, "Failed to create session")

	suite.container, err = suite.session.NewContainer("1024")
	suite.Require().NoError(err, "Failed to create container")
}

//
// API tests (all commands and such)
//

type SyncContextApiTestSuite struct {
	SyncContextTestSuite
}

//func (suite *SyncContextApiTestSuite) TestListAll() {
//
//	// get all buckets
//	response, err := suite.container.Sync.ListAll()
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

func (suite *SyncContextApiTestSuite) TestObject() {
	// suite.T().Skip()

	path := "object.txt"
	contents := "vegans are better than everyone"

	//
	// PUT contents to some object
	//

	err := suite.container.Sync.PutObject(&PutObjectInput{
		Path: path,
		Body: []byte(contents),
	})

	suite.Require().NoError(err, "Failed to put")

	//
	// Get the contents
	//

	response, err := suite.container.Sync.GetObject(&GetObjectInput{
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

	err = suite.container.Sync.DeleteObject(&DeleteObjectInput{
		Path: path,
	})

	suite.Require().NoError(err, "Failed to delete")

	//
	// Get the contents again (should fail)
	//

	response, err = suite.container.Sync.GetObject(&GetObjectInput{
		Path: path,
	})

	suite.Require().Error(err, "Failed to get")
	suite.Require().Nil(response)
}

func (suite *SyncContextApiTestSuite) TestEMD() {
	// suite.T().Skip()

	items := map[string]map[string]interface{}{
		"bob":    {"age": 42, "feature": "mustance"},
		"linda":  {"age": 41, "feature": "singing"},
		"louise": {"age": 9, "feature": "bunny ears"},
		"tina":   {"age": 14, "feature": "butts"},
	}

	//
	// Create and update items
	//

	// create the items
	for itemKey, itemAttributes := range items {
		input := PutItemInput{
			Path:       "emd0/" + itemKey,
			Attributes: itemAttributes,
		}

		// get a specific bucket
		err := suite.container.Sync.PutItem(&input)
		suite.Require().NoError(err, "Failed to put item")
	}

	// update louise item
	updateItemInput := UpdateItemInput{
		Path: "emd0/louise",
		Attributes: map[string]interface{}{
			"height": 130,
			"quip":   "i can smell fear on you",
		},
	}

	err := suite.container.Sync.UpdateItem(&updateItemInput)
	suite.Require().NoError(err, "Failed to update item")

	//
	// Get item(s)
	//

	// get tina
	getItemInput := GetItemInput{
		Path:           "emd0/louise",
		AttributeNames: []string{"__size", "age", "quip", "height"},
	}

	response, err := suite.container.Sync.GetItem(&getItemInput)
	suite.Require().NoError(err, "Failed to get item")

	getItemOutput := response.Output.(*GetItemOutput)

	// make sure we got the age and quip correctly
	suite.Require().Equal(0, getItemOutput.Item["__size"].(int))
	suite.Require().Equal(130, getItemOutput.Item["height"].(int))
	suite.Require().Equal("i can smell fear on you", getItemOutput.Item["quip"].(string))
	suite.Require().Equal(9, getItemOutput.Item["age"].(int))

	// release the response
	response.Release()

	// get all items whose age is over 15
	getItemsInput := GetItemsInput{
		Path:           "emd0/",
		AttributeNames: []string{"age", "feature"},
		Filter:         "age > 15",
	}

	response, err = suite.container.Sync.GetItems(&getItemsInput)
	suite.Require().NoError(err, "Failed to get items")

	getItemsOutput := response.Output.(*GetItemsOutput)
	suite.Require().Len(getItemsOutput.Items, 2)

	// iterate over age, make sure it's over 15
	for _, item := range getItemsOutput.Items {
		suite.Require().True(item["age"].(int) > 15)
	}

	// release the response
	response.Release()

	//
	// Increment age
	//

	incrementAgeExpression := "age = age + 1"

	// update louise's age
	updateItemInput = UpdateItemInput{
		Path:       "emd0/louise",
		Expression: &incrementAgeExpression,
	}

	err = suite.container.Sync.UpdateItem(&updateItemInput)
	suite.Require().NoError(err, "Failed to update item")

	// get tina
	getItemInput = GetItemInput{
		Path:           "emd0/louise",
		AttributeNames: []string{"age"},
	}

	response, err = suite.container.Sync.GetItem(&getItemInput)
	suite.Require().NoError(err, "Failed to get item")

	getItemOutput = response.Output.(*GetItemOutput)

	// check that age incremented
	suite.Require().Equal(10, getItemOutput.Item["age"].(int))

	// release the response
	response.Release()

	//
	// Delete everything
	//

	// delete the items
	for itemKey, _ := range items {
		input := DeleteObjectInput{
			Path: "emd0/" + itemKey,
		}

		// get a specific bucket
		err := suite.container.Sync.DeleteObject(&input)
		suite.Require().NoError(err, "Failed to delete item")
	}

	// delete the directory
	err = suite.container.Sync.DeleteObject(&DeleteObjectInput{
		Path: "emd0/",
	})

	suite.Require().NoError(err, "Failed to delete")
}

//
// Stress test
//

type SyncContextStressTestSuite struct {
	SyncContextTestSuite
}

func (suite *SyncContextStressTestSuite) TestStressPutGet() {
	pathTemplate := "stress/stress-%d.txt"
	contents := "0123456789"

	waitGroup := sync.WaitGroup{}

	// spawn workers - each putting / getting a different object
	for workerIndex := 0; workerIndex < 32; workerIndex++ {
		waitGroup.Add(1)

		go func(workerIndex int) {
			path := fmt.Sprintf(pathTemplate, workerIndex)

			for iteration := 0; iteration < 50; iteration++ {

				err := suite.container.Sync.PutObject(&PutObjectInput{
					Path: path,
					Body: []byte(contents),
				})

				suite.Require().NoError(err, "Failed to put")

				response, err := suite.container.Sync.GetObject(&GetObjectInput{
					Path: path,
				})

				suite.Require().NoError(err, "Failed to get")

				// release the response
				response.Release()
			}

			// delete the object
			err := suite.container.Sync.DeleteObject(&DeleteObjectInput{
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
func TestSyncContextTestSuite(t *testing.T) {
	suite.Run(t, new(SyncContextApiTestSuite))
	// suite.Run(t, new(SyncContextStressTestSuite))
}
