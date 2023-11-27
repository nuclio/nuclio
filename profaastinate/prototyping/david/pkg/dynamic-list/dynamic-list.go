package dynamicList

import (
	"sync"
	"time"
)

// Priority is used to specify the priority of an item.
type Priority string

// Corresponding constants for the Priority type.
const (
	PriorityPriority = "Priority"
	PriorityDeadline = "Deadline"
	PriorityName     = "Name"
)

// IntPriority Maps the Priority type to an integer for indexing.
var IntPriority = map[int]Priority{
	0: PriorityPriority,
	1: PriorityDeadline,
	2: PriorityName,
}

// Item represents an item in the list.
type Item struct {
	// The raw value of the item.
	RawValue interface{}
	Attrs    map[Priority]interface{}
	compare  func(a, b interface{}) bool
}

// List represents a resizable list.
type List []*Item

// Push adds an item to the list.
func (l *List) Push(item *Item) {
	*l = append(*l, item)
}

// Returns the value of the given dynamic property.
func getValue(obj interface{}, property Priority) interface{} {
	if val, ok := obj.(map[Priority]interface{})[property]; ok {
		return val
	}
	return nil
}

func (l *List) Len() int {
	return len(*l)
}

// PopItemByType selects which priority to pop by based on the type.
func (l *List) PopItemByType(property Priority) *Item {
	return l.pop(func(a, b interface{}) bool {
		valA := getValue(a, property)
		valB := getValue(b, property)

		switch valA.(type) {
		case uint:
			return valA.(float64) > valB.(float64)
		case int:
			return valA.(int) > valB.(int)
		case float64:
			return valA.(float64) > valB.(float64)
		case string:
			return valA.(string) < valB.(string)
		case time.Time:
			return valA.(time.Time).Before(valB.(time.Time))
		default:
			return false
		}
	})
}

// Pop removes and returns the item with the highest priority.
func (l *List) pop(compareFunc func(a, b interface{}) bool) *Item {
	if len(*l) == 0 {
		return nil
	}

	var bestItem *Item
	for _, item := range *l {
		if bestItem == nil || compareFunc(item.Attrs, bestItem.Attrs) {
			bestItem = item
		}
	}

	// Remove the best item from the list
	for i, item := range *l {
		if item == bestItem {
			*l = append((*l)[:i], (*l)[i+1:]...)
			break
		}
	}

	return bestItem
}

// Govern the popping of items
func PopItems(list *List, attr Priority, ch chan *Item, wg *sync.WaitGroup) {
	defer wg.Done()

	for list.Len() > 0 {
		var item *Item
		item = list.PopItemByType(attr)

		if item != nil {
			ch <- item
		}
	}
}

// Print the popped items
func PrintPoppedItems(ch chan *Item, attr Priority) {
	for _ = range ch {
		// fmt.Printf("Popped by %s: %v, Attrs: %v\n", attr, item.RawValue, item.Attrs)
	}
}
