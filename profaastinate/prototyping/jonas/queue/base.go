package queue

import (
	"time"
)

// Queue structure where position is based on a deadline
type Queue struct {
	items []Item
}

// Item structure with a deadline
type Item struct {
	Value    interface{}
	deadline time.Time
}

func NewItem(value interface{}, deadline time.Time) *Item {
	return &Item{
		Value:    value,
		deadline: deadline,
	}
}

// Function to initialize the queue
func InitQueue() *Queue {
	return &Queue{
		items: make([]Item, 0),
	}
}

func (q *Queue) Len() int {
	return len(q.items)
}

// Function to add items to the queue
func (q *Queue) Add(item Item) {
	q.items = append(q.items, item)
}

// Function to pop entries from the queue based on a timeframe
func (q *Queue) Pop(timeframe time.Time) []Item {
	var poppedItems []Item
	for i, item := range q.items {
		if item.deadline.Before(timeframe) {
			// this needs to be calculated because the slice is being modified between iterations
			itemsLocation := i - len(poppedItems)

			poppedItems = append(poppedItems, item)

			// swap the item to be removed with the last item in the slice
			// this avoids heavy slice operations
			// this is faster than using append() to remove the item
			// append is O(n) because it copies the entire slice
			// now we have O(1) because we are just swapping the item to be removed with the first item in the slice
			// and then we are removing the last item in the slice
			// this is a constant time operation
			// https://stackoverflow.com/questions/37334119/what-is-the-time-complexity-of-appending-to-a-slice-in-golang
			q.items[itemsLocation] = q.items[len(q.items)-1]
			q.items = q.items[:len(q.items)-1]
		}
	}

	println(len(q.items))
	println(len(poppedItems))
	return poppedItems
}

// Function to inspect some entries for a given time frame, or all items
func (q *Queue) Inspect(timeframe time.Time) []Item {
	var inspectedItems []Item
	for _, item := range q.items {
		if item.deadline.Before(timeframe) {
			inspectedItems = append(inspectedItems, item)
		}
	}
	return inspectedItems
}

func (q *Queue) InspectAll() []Item {
	return q.items
}

// Function to remove specific entries from the queue
func (q *Queue) Remove(itemToRemove Item) {
	for i, item := range q.items {
		if item == itemToRemove {
			q.items[i] = q.items[len(q.items)-1]
			q.items = q.items[:len(q.items)-1]
			break
		}
	}
}
