package v3iohttp

import (
	"github.com/v3io/v3io-go/pkg/dataplane"

	"github.com/nuclio/logger"
)

type container struct {
	logger        logger.Logger
	session       *session
	containerName string
}

func newContainer(parentLogger logger.Logger,
	session *session,
	containerName string) (v3io.Container, error) {

	return &container{
		logger:        parentLogger.GetChild("container"),
		session:       session,
		containerName: containerName,
	}, nil
}

func (c *container) populateInputFields(input *v3io.DataPlaneInput) {
	input.ContainerName = c.containerName
	input.URL = c.session.url
	input.AuthenticationToken = c.session.authenticationToken
	input.AccessKey = c.session.accessKey
}

// GetItem
func (c *container) GetItem(getItemInput *v3io.GetItemInput,
	context interface{},
	responseChan chan *v3io.Response) (*v3io.Request, error) {
	c.populateInputFields(&getItemInput.DataPlaneInput)
	return c.session.context.GetItem(getItemInput, context, responseChan)
}

// GetItemSync
func (c *container) GetItemSync(getItemInput *v3io.GetItemInput) (*v3io.Response, error) {
	c.populateInputFields(&getItemInput.DataPlaneInput)
	return c.session.context.GetItemSync(getItemInput)
}

// GetItems
func (c *container) GetItems(getItemsInput *v3io.GetItemsInput,
	context interface{},
	responseChan chan *v3io.Response) (*v3io.Request, error) {
	c.populateInputFields(&getItemsInput.DataPlaneInput)
	return c.session.context.GetItems(getItemsInput, context, responseChan)
}

// GetItemSync
func (c *container) GetItemsSync(getItemsInput *v3io.GetItemsInput) (*v3io.Response, error) {
	c.populateInputFields(&getItemsInput.DataPlaneInput)
	return c.session.context.GetItemsSync(getItemsInput)
}

// PutItem
func (c *container) PutItem(putItemInput *v3io.PutItemInput,
	context interface{},
	responseChan chan *v3io.Response) (*v3io.Request, error) {
	c.populateInputFields(&putItemInput.DataPlaneInput)
	return c.session.context.PutItem(putItemInput, context, responseChan)
}

// PutItemSync
func (c *container) PutItemSync(putItemInput *v3io.PutItemInput) error {
	c.populateInputFields(&putItemInput.DataPlaneInput)
	return c.session.context.PutItemSync(putItemInput)
}

// PutItems
func (c *container) PutItems(putItemsInput *v3io.PutItemsInput,
	context interface{},
	responseChan chan *v3io.Response) (*v3io.Request, error) {
	c.populateInputFields(&putItemsInput.DataPlaneInput)
	return c.session.context.PutItems(putItemsInput, context, responseChan)
}

// PutItemsSync
func (c *container) PutItemsSync(putItemsInput *v3io.PutItemsInput) (*v3io.Response, error) {
	c.populateInputFields(&putItemsInput.DataPlaneInput)
	return c.session.context.PutItemsSync(putItemsInput)
}

// UpdateItem
func (c *container) UpdateItem(updateItemInput *v3io.UpdateItemInput,
	context interface{},
	responseChan chan *v3io.Response) (*v3io.Request, error) {
	c.populateInputFields(&updateItemInput.DataPlaneInput)
	return c.session.context.UpdateItem(updateItemInput, context, responseChan)
}

// UpdateItemSync
func (c *container) UpdateItemSync(updateItemInput *v3io.UpdateItemInput) error {
	c.populateInputFields(&updateItemInput.DataPlaneInput)
	return c.session.context.UpdateItemSync(updateItemInput)
}

// GetObject
func (c *container) GetObject(getObjectInput *v3io.GetObjectInput,
	context interface{},
	responseChan chan *v3io.Response) (*v3io.Request, error) {
	c.populateInputFields(&getObjectInput.DataPlaneInput)
	return c.session.context.GetObject(getObjectInput, context, responseChan)
}

// GetObjectSync
func (c *container) GetObjectSync(getObjectInput *v3io.GetObjectInput) (*v3io.Response, error) {
	c.populateInputFields(&getObjectInput.DataPlaneInput)
	return c.session.context.GetObjectSync(getObjectInput)
}

// PutObject
func (c *container) PutObject(putObjectInput *v3io.PutObjectInput,
	context interface{},
	responseChan chan *v3io.Response) (*v3io.Request, error) {
	c.populateInputFields(&putObjectInput.DataPlaneInput)
	return c.session.context.PutObject(putObjectInput, context, responseChan)
}

// PutObjectSync
func (c *container) PutObjectSync(putObjectInput *v3io.PutObjectInput) error {
	c.populateInputFields(&putObjectInput.DataPlaneInput)
	return c.session.context.PutObjectSync(putObjectInput)
}

// DeleteObject
func (c *container) DeleteObject(deleteObjectInput *v3io.DeleteObjectInput,
	context interface{},
	responseChan chan *v3io.Response) (*v3io.Request, error) {
	c.populateInputFields(&deleteObjectInput.DataPlaneInput)
	return c.session.context.DeleteObject(deleteObjectInput, context, responseChan)
}

// DeleteObjectSync
func (c *container) DeleteObjectSync(deleteObjectInput *v3io.DeleteObjectInput) error {
	c.populateInputFields(&deleteObjectInput.DataPlaneInput)
	return c.session.context.DeleteObjectSync(deleteObjectInput)
}

// GetContainers
func (c *container) GetContainers(getContainersInput *v3io.GetContainersInput, context interface{}, responseChan chan *v3io.Response) (*v3io.Request, error) {
	c.populateInputFields(&getContainersInput.DataPlaneInput)
	return c.session.context.GetContainers(getContainersInput, context, responseChan)
}

// GetContainersSync
func (c *container) GetContainersSync(getContainersInput *v3io.GetContainersInput) (*v3io.Response, error) {
	c.populateInputFields(&getContainersInput.DataPlaneInput)
	return c.session.context.GetContainersSync(getContainersInput)
}

// GetContainers
func (c *container) GetContainerContents(getContainerContentsInput *v3io.GetContainerContentsInput, context interface{}, responseChan chan *v3io.Response) (*v3io.Request, error) {
	c.populateInputFields(&getContainerContentsInput.DataPlaneInput)
	return c.session.context.GetContainerContents(getContainerContentsInput, context, responseChan)
}

// GetContainerContentsSync
func (c *container) GetContainerContentsSync(getContainerContentsInput *v3io.GetContainerContentsInput) (*v3io.Response, error) {
	c.populateInputFields(&getContainerContentsInput.DataPlaneInput)
	return c.session.context.GetContainerContentsSync(getContainerContentsInput)
}

// CreateStream
func (c *container) CreateStream(createStreamInput *v3io.CreateStreamInput, context interface{}, responseChan chan *v3io.Response) (*v3io.Request, error) {
	c.populateInputFields(&createStreamInput.DataPlaneInput)
	return c.session.context.CreateStream(createStreamInput, context, responseChan)
}

// CreateStreamSync
func (c *container) CreateStreamSync(createStreamInput *v3io.CreateStreamInput) error {
	c.populateInputFields(&createStreamInput.DataPlaneInput)
	return c.session.context.CreateStreamSync(createStreamInput)
}

// DescribeStream
func (c *container) DescribeStream(describeStreamInput *v3io.DescribeStreamInput,
	context interface{},
	responseChan chan *v3io.Response) (*v3io.Request, error) {
	c.populateInputFields(&describeStreamInput.DataPlaneInput)
	return c.session.context.DescribeStream(describeStreamInput, context, responseChan)
}

// DescribeStreamSync
func (c *container) DescribeStreamSync(describeStreamInput *v3io.DescribeStreamInput) (*v3io.Response, error) {
	c.populateInputFields(&describeStreamInput.DataPlaneInput)
	return c.session.context.DescribeStreamSync(describeStreamInput)
}

// DeleteStream
func (c *container) DeleteStream(deleteStreamInput *v3io.DeleteStreamInput, context interface{}, responseChan chan *v3io.Response) (*v3io.Request, error) {
	c.populateInputFields(&deleteStreamInput.DataPlaneInput)
	return c.session.context.DeleteStream(deleteStreamInput, context, responseChan)
}

// DeleteStreamSync
func (c *container) DeleteStreamSync(deleteStreamInput *v3io.DeleteStreamInput) error {
	c.populateInputFields(&deleteStreamInput.DataPlaneInput)
	return c.session.context.DeleteStreamSync(deleteStreamInput)
}

// SeekShard
func (c *container) SeekShard(seekShardInput *v3io.SeekShardInput, context interface{}, responseChan chan *v3io.Response) (*v3io.Request, error) {
	c.populateInputFields(&seekShardInput.DataPlaneInput)
	return c.session.context.SeekShard(seekShardInput, context, responseChan)
}

// SeekShardSync
func (c *container) SeekShardSync(seekShardInput *v3io.SeekShardInput) (*v3io.Response, error) {
	c.populateInputFields(&seekShardInput.DataPlaneInput)
	return c.session.context.SeekShardSync(seekShardInput)
}

// PutRecords
func (c *container) PutRecords(putRecordsInput *v3io.PutRecordsInput, context interface{}, responseChan chan *v3io.Response) (*v3io.Request, error) {
	c.populateInputFields(&putRecordsInput.DataPlaneInput)
	return c.session.context.PutRecords(putRecordsInput, context, responseChan)
}

// PutRecordsSync
func (c *container) PutRecordsSync(putRecordsInput *v3io.PutRecordsInput) (*v3io.Response, error) {
	c.populateInputFields(&putRecordsInput.DataPlaneInput)
	return c.session.context.PutRecordsSync(putRecordsInput)
}

// GetRecords
func (c *container) GetRecords(getRecordsInput *v3io.GetRecordsInput, context interface{}, responseChan chan *v3io.Response) (*v3io.Request, error) {
	c.populateInputFields(&getRecordsInput.DataPlaneInput)
	return c.session.context.GetRecords(getRecordsInput, context, responseChan)
}

// GetRecordsSync
func (c *container) GetRecordsSync(getRecordsInput *v3io.GetRecordsInput) (*v3io.Response, error) {
	c.populateInputFields(&getRecordsInput.DataPlaneInput)
	return c.session.context.GetRecordsSync(getRecordsInput)
}
