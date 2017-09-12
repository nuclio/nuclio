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

	suite.logger, err = nucliozap.NewNuclioZapTest("test")

	suite.context, err = NewContext(suite.logger, "192.168.51.240:8081", 1)
	suite.Require().NoError(err, "Failed to create context")

	suite.session, err = suite.context.NewSession("iguazio", "iguazio", "iguazio")
	suite.Require().NoError(err, "Failed to create session")

	suite.container, err = suite.session.NewContainer("1024")
	suite.Require().NoError(err, "Failed to create container")
}

//
// Object tests
//

type SyncContextObjectTestSuite struct {
	SyncContextTestSuite
}

func (suite *SyncContextObjectTestSuite) TestObject() {
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

//
// EMD tests
//

type SyncContextEMDTestSuite struct {
	SyncContextTestSuite
}

func (suite *SyncContextEMDTestSuite) TestEMD() {
	items := map[string]map[string]interface{}{
		"bob":    {"age": 42, "feature": "mustache"},
		"linda":  {"age": 41, "feature": "singing"},
		"louise": {"age": 9, "feature": "bunny ears"},
		"tina":   {"age": 14, "feature": "butts"},
	}

	//
	// Create items one by one
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

	suite.verifyItems(items)

	//
	// Update item and verify
	//

	condition := "age > 5"

	// update louise item
	updateItemInput := UpdateItemInput{
		Path: "emd0/louise",
		Attributes: map[string]interface{}{
			"height": 130,
			"quip":   "i can smell fear on you",
		},
		Condition: &condition,
	}

	err := suite.container.Sync.UpdateItem(&updateItemInput)
	suite.Require().NoError(err, "Failed to update item")

	// get louise
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

	suite.deleteItems(items)
}

func (suite *SyncContextEMDTestSuite) TestPutItems() {
	items := map[string]map[string]interface{}{
		"bob":   {"age": 42, "feature": "mustache"},
		"linda": {"age": 41, "feature": "singing"},
	}

	// get a specific bucket
	response, err := suite.container.Sync.PutItems(&PutItemsInput{
		Path:  "emd0",
		Items: items,
	})
	suite.Require().NoError(err, "Failed to put items")

	putItemsOutput := response.Output.(*PutItemsOutput)

	// must succeed - everything was valid
	suite.Require().True(putItemsOutput.Success)
	suite.Require().Nil(putItemsOutput.Errors)

	response.Release()

	suite.verifyItems(items)

	suite.deleteItems(items)
}

func (suite *SyncContextEMDTestSuite) TestPutItemsWithError() {
	items := map[string]map[string]interface{}{
		"bob":     {"age": 42, "feature": "mustache"},
		"linda":   {"age": 41, "feature": "singing"},
		"invalid": {"__name": "foo", "feature": "singing"},
	}

	// get a specific bucket
	response, err := suite.container.Sync.PutItems(&PutItemsInput{
		Path:  "emd0",
		Items: items,
	})
	suite.Require().NoError(err, "Failed to put items")

	putItemsOutput := response.Output.(*PutItemsOutput)

	// must succeed - everything was valid
	suite.Require().False(putItemsOutput.Success)
	suite.Require().NotNil(putItemsOutput.Errors)
	suite.Require().NotNil(putItemsOutput.Errors["invalid"])

	response.Release()

	// remove invalid because it shouldn't be verified / deleted
	delete(items, "invalid")

	suite.verifyItems(items)

	suite.deleteItems(items)
}

func (suite *SyncContextEMDTestSuite) verifyItems(items map[string]map[string]interface{}) {

	// get all items
	getItemsInput := GetItemsInput{
		Path:           "emd0/",
		AttributeNames: []string{"*"},
	}

	response, err := suite.container.Sync.GetItems(&getItemsInput)
	suite.Require().NoError(err, "Failed to get items")

	getItemsOutput := response.Output.(*GetItemsOutput)
	suite.Require().Len(getItemsOutput.Items, len(items))

	// TODO: test values

	// release the response
	response.Release()
}

func (suite *SyncContextEMDTestSuite) deleteItems(items map[string]map[string]interface{}) {

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
	err := suite.container.Sync.DeleteObject(&DeleteObjectInput{
		Path: "emd0/",
	})

	suite.Require().NoError(err, "Failed to delete")
}

//
// Stream tests
//

type SyncContextStreamTestSuite struct {
	SyncContextTestSuite
	testPath string
}

func (suite *SyncContextStreamTestSuite) SetupTest() {
	suite.SyncContextTestSuite.SetupTest()

	suite.testPath = "stream-test"

	suite.deleteAllStreamsInPath(suite.testPath)
}

func (suite *SyncContextStreamTestSuite) TearDownTest() {
	suite.deleteAllStreamsInPath(suite.testPath)
}

func (suite *SyncContextStreamTestSuite) TestStream() {
	streamPath := fmt.Sprintf("%s/mystream/", suite.testPath)

	//
	// Create the stream
	//

	err := suite.container.Sync.CreateStream(&CreateStreamInput{
		Path:                 streamPath,
		ShardCount:           4,
		RetentionPeriodHours: 1,
	})

	suite.Require().NoError(err, "Failed to create stream")

	//
	// Put some records
	//

	firstShardID := 1
	secondShardID := 2
	invalidShardID := 10

	records := []*StreamRecord{
		{ShardID: &firstShardID, Data: []byte("first shard record #1")},
		{ShardID: &firstShardID, Data: []byte("first shard record #2")},
		{ShardID: &invalidShardID, Data: []byte("invalid shard record #1")},
		{ShardID: &secondShardID, Data: []byte("second shard record #1")},
		{Data: []byte("some shard record #1")},
	}

	response, err := suite.container.Sync.PutRecords(&PutRecordsInput{
		Path:    streamPath,
		Records: records,
	})
	suite.Require().NoError(err, "Failed to put records")

	putRecordsResponse := response.Output.(*PutRecordsOutput)

	// should have one failure
	suite.Require().Equal(1, putRecordsResponse.FailedRecordCount)

	// verify record results
	for recordIdx, record := range putRecordsResponse.Records {

		// third record should've failed
		if recordIdx == 2 {
			suite.Require().NotEqual(0, record.ErrorCode)
		} else {
			suite.Require().Equal(0, record.ErrorCode)
		}
	}

	response.Release()

	//
	// Seek
	//

	response, err = suite.container.Sync.SeekShard(&SeekShardInput{
		Path: streamPath + "1",
		Type: SeekShardInputTypeEarliest,
	})

	suite.Require().NoError(err, "Failed to seek shard")
	location := response.Output.(*SeekShardOutput).Location

	suite.Require().NotEqual("", location)

	response.Release()

	//
	// Get records
	//

	response, err = suite.container.Sync.GetRecords(&GetRecordsInput{
		Path:     streamPath + "1",
		Location: location,
		Limit:    100,
	})

	suite.Require().NoError(err, "Failed to get records")

	getRecordsOutput := response.Output.(*GetRecordsOutput)

	suite.Require().Equal("first shard record #1", string(getRecordsOutput.Records[0].Data))
	suite.Require().Equal("first shard record #2", string(getRecordsOutput.Records[1].Data))

	response.Release()

	//
	// Delete stream
	//

	err = suite.container.Sync.DeleteStream(&DeleteStreamInput{
		Path: streamPath,
	})
	suite.Require().NoError(err, "Failed to delete stream")
}

func (suite *SyncContextStreamTestSuite) deleteAllStreamsInPath(path string) error {

	// get all streams in the test path
	response, err := suite.container.Sync.ListBucket(&ListBucketInput{
		Path: path,
	})

	if err != nil {
		return err
	}

	defer response.Release()

	// iterate over streams (prefixes) and delete them
	for _, commonPrefix := range response.Output.(*ListBucketOutput).CommonPrefixes {

		suite.container.Sync.DeleteStream(&DeleteStreamInput{
			Path: commonPrefix.Prefix,
		})
	}

	return nil
}

//
// Cursor test
//

type SyncContextCursorTestSuite struct {
	SyncContextTestSuite
	numItems int
}

func (suite *SyncContextCursorTestSuite) SetupTest() {
	suite.SyncContextTestSuite.SetupTest()

	suite.numItems = 50

	// create N items
	for itemIndex := 0; itemIndex < suite.numItems; itemIndex++ {
		input := PutItemInput{
			Path:       suite.getItemKey(itemIndex),
			Attributes: map[string]interface{}{"attr": itemIndex},
		}

		// get a specific bucket
		err := suite.container.Sync.PutItem(&input)
		suite.Require().NoError(err, "Failed to put item")
	}
}

func (suite *SyncContextCursorTestSuite) TearDownTest() {
	for itemIndex := 0; itemIndex < suite.numItems; itemIndex++ {
		input := DeleteObjectInput{
			Path: suite.getItemKey(itemIndex),
		}

		// get a specific bucket
		err := suite.container.Sync.DeleteObject(&input)
		suite.Require().NoError(err, "Failed to delete item")
	}
}

func (suite *SyncContextCursorTestSuite) TestEMDCursorNoEntries() {
	// suite.T().Skip()

	getItemsInput := GetItemsInput{
		Path:           "emd0",
		AttributeNames: []string{"*"},
		Filter:         "attr > 100000",
	}

	cursor, err := suite.container.Sync.GetItemsCursor(&getItemsInput)
	suite.Require().NoError(err, "Failed to get items")

	item, err := cursor.Next()
	suite.Require().NoError(err)
	suite.Require().Nil(item)

	cursor.Release()
}

func (suite *SyncContextCursorTestSuite) TestEMDCursorNext() {
	// suite.T().Skip()

	getItemsInput := GetItemsInput{
		Path:           "emd0",
		AttributeNames: []string{"*"},
		Limit:          5,
	}

	cursor, err := suite.container.Sync.GetItemsCursor(&getItemsInput)
	suite.Require().NoError(err, "Failed to get items")

	for itemIndex := 0; itemIndex < suite.numItems; itemIndex++ {
		item, err := cursor.Next()
		suite.Require().NoError(err)
		suite.Require().NotNil(item)

		suite.verifyItem(item)
	}

	cursor.Release()
}

func (suite *SyncContextCursorTestSuite) TestEMDCursorAll() {
	// suite.T().Skip()

	getItemsInput := GetItemsInput{
		Path:           "emd0",
		AttributeNames: []string{"*"},
		Limit:          5,
	}

	cursor, err := suite.container.Sync.GetItemsCursor(&getItemsInput)
	suite.Require().NoError(err, "Failed to get items cursor")

	items, err := cursor.All()
	suite.Require().NoError(err, "Failed to get all items")

	suite.Require().Len(items, suite.numItems)

	// verify values
	for _, item := range items {
		suite.verifyItem(item)
	}

	cursor.Release()
}

func (suite *SyncContextCursorTestSuite) getItemKey(itemIndex int) string {
	return fmt.Sprintf("emd0/item-%d", itemIndex)
}

func (suite *SyncContextCursorTestSuite) verifyItem(item *Item) {
	suite.Require().Equal((*item)["__name"].(string), fmt.Sprintf("item-%d", (*item)["attr"]))
}

//
// Stress test
//

type SyncContextStressTestSuite struct {
	SyncContextTestSuite
}

func (suite *SyncContextStressTestSuite) TestStressPutGet() {
	// suite.T().Skip()

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
	suite.Run(t, new(SyncContextObjectTestSuite))
	suite.Run(t, new(SyncContextEMDTestSuite))
	suite.Run(t, new(SyncContextStreamTestSuite))
	suite.Run(t, new(SyncContextCursorTestSuite))
	suite.Run(t, new(SyncContextStressTestSuite))
}
