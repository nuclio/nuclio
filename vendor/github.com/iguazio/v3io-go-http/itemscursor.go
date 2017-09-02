package v3io

type ItemsCursor struct {
	Sync *SyncItemsCursor
}

func newItemsCursor(container *Container, input *GetItemsInput, response *Response) *ItemsCursor {
	return &ItemsCursor{}
}

// TODO: support Next and All() for async as well
