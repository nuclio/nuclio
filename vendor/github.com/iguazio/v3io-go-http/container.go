package v3io

import (
	"github.com/pkg/errors"
	"sync/atomic"
)

// TODO: Request should have a global pool
var requestID uint64 = 0

type Container struct {
	logger  Logger
	session *Session
	Sync    *SyncContainer
}

func newContainer(parentLogger Logger, session *Session, alias string) (*Container, error) {
	newSyncContainer, err := newSyncContainer(parentLogger, session.Sync, alias)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create sync container")
	}

	return &Container{
		logger:  parentLogger.GetChild(alias).(Logger),
		session: session,
		Sync:    newSyncContainer,
	}, nil
}

func (c *Container) ListAll(input *ListAllInput, responseChan chan *Response) (*Request, error) {
	return c.sendRequest(input, responseChan)
}

func (c *Container) ListBucket(input *ListBucketInput, responseChan chan *Response) (*Request, error) {
	return c.sendRequest(input, responseChan)
}

func (c *Container) GetObject(input *GetObjectInput, responseChan chan *Response) (*Request, error) {
	return c.sendRequest(input, responseChan)
}

func (c *Container) DeleteObject(input *DeleteObjectInput, responseChan chan *Response) (*Request, error) {
	return c.sendRequest(input, responseChan)
}

func (c *Container) PutObject(input *PutObjectInput, responseChan chan *Response) (*Request, error) {
	return c.sendRequest(input, responseChan)
}

func (c *Container) GetItem(input *GetItemInput, responseChan chan *Response) (*Request, error) {
	return c.sendRequest(input, responseChan)
}

func (c *Container) GetItems(input *GetItemsInput, responseChan chan *Response) (*Request, error) {
	return c.sendRequest(input, responseChan)
}

func (c *Container) PutItem(input *PutItemInput, responseChan chan *Response) (*Request, error) {
	return c.sendRequest(input, responseChan)
}

func (c *Container) UpdateItem(input *UpdateItemInput, responseChan chan *Response) (*Request, error) {
	return c.sendRequest(input, responseChan)
}

func (c *Container) sendRequest(input interface{}, responseChan chan *Response) (*Request, error) {
	id := atomic.AddUint64(&requestID, 1)

	// create a request/response (TODO: from pool)
	requestResponse := &RequestResponse{
		Request: Request{
			ID:           id,
			container:    c,
			input:        input,
			responseChan: responseChan,
		},
	}

	// point to container
	requestResponse.Request.RequestResponse = requestResponse

	if err := c.session.sendRequest(&requestResponse.Request); err != nil {
		return nil, errors.Wrap(err, "Failed to send request")
	}

	return &requestResponse.Request, nil
}
