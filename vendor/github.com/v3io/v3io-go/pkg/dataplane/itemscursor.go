package v3io

type ItemsCursor struct {
	currentItem     Item
	currentError    error
	currentResponse *Response
	nextMarker      string
	moreItemsExist  bool
	itemIndex       int
	items           []Item
	getItemsInput   *GetItemsInput
	container       Container
}

func NewItemsCursor(container Container, getItemsInput *GetItemsInput) (*ItemsCursor, error) {
	newItemsCursor := &ItemsCursor{
		container:     container,
		getItemsInput: getItemsInput,
	}

	response, err := container.GetItemsSync(getItemsInput)
	if err != nil {
		return nil, err
	}

	newItemsCursor.setResponse(response)

	return newItemsCursor, nil
}

// Err returns the last error
func (ic *ItemsCursor) Err() error {
	return ic.currentError
}

// Release releases a cursor and its underlying resources
func (ic *ItemsCursor) Release() {
	if ic.currentResponse != nil {
		ic.currentResponse.Release()
	}
}

// Next gets the next matching item. this may potentially block as this lazy loads items from the collection
func (ic *ItemsCursor) NextSync() bool {
	item, err := ic.NextItemSync()

	if item == nil || err != nil {
		return false
	}

	return true
}

// NextItem gets the next matching item. this may potentially block as this lazy loads items from the collection
func (ic *ItemsCursor) NextItemSync() (Item, error) {

	// are there any more items left in the previous response we received?
	if ic.itemIndex < len(ic.items) {
		ic.currentItem = ic.items[ic.itemIndex]
		ic.currentError = nil

		// next time we'll give next item
		ic.itemIndex++

		return ic.currentItem, nil
	}

	// are there any more items up stream?
	if !ic.moreItemsExist {
		ic.currentError = nil
		return nil, nil
	}

	// get the previous request input and modify it with the marker
	ic.getItemsInput.Marker = ic.nextMarker

	// invoke get items
	newResponse, err := ic.container.GetItemsSync(ic.getItemsInput)
	if err != nil {
		return nil, err
	}

	// release the previous response
	ic.currentResponse.Release()

	// set the new response - read all the sub information from it
	ic.setResponse(newResponse)

	// and recurse into next now that we repopulated response
	return ic.NextItemSync()
}

// gets all items
func (ic *ItemsCursor) AllSync() ([]Item, error) {
	var items []Item

	for ic.NextSync() {
		items = append(items, ic.GetItem())
	}

	if ic.Err() != nil {
		return nil, ic.Err()
	}

	return items, nil
}

func (ic *ItemsCursor) GetField(name string) interface{} {
	return ic.currentItem[name]
}

func (ic *ItemsCursor) GetFieldInt(name string) (int, error) {
	return ic.currentItem.GetFieldInt(name)
}

func (ic *ItemsCursor) GetFieldString(name string) (string, error) {
	return ic.currentItem.GetFieldString(name)
}

func (ic *ItemsCursor) GetFields() map[string]interface{} {
	return ic.currentItem
}

func (ic *ItemsCursor) GetItem() Item {
	return ic.currentItem
}

func (ic *ItemsCursor) setResponse(response *Response) {
	ic.currentResponse = response

	getItemsOutput := response.Output.(*GetItemsOutput)

	ic.moreItemsExist = !getItemsOutput.Last
	ic.nextMarker = getItemsOutput.NextMarker
	ic.items = getItemsOutput.Items
	ic.itemIndex = 0
}
