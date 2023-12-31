package common

import (
	common "github.com/konsumgandalf/profaastinate/nexus/common/models/structs"
	"testing"
	"time"
)

func TestPriorityQueue(t *testing.T) {
	mockPriorityQueue := Initialize()

	startTime := time.Now()

	mockItemList := []*common.NexusItem{
		{
			Index:    0,
			Deadline: startTime,
		},
		{
			Index:    1,
			Deadline: startTime.Add(20 * time.Second),
		},
	}
	firstItem := mockItemList[0]

	// Test Push
	for _, item := range mockItemList {
		mockPriorityQueue.Push(item)
	}
	if mockPriorityQueue.Len() != 2 {
		t.Errorf("Expected length 2, got %d", mockPriorityQueue.Len())
	}

	// Test Peek
	if mockPriorityQueue.Peek() != firstItem {
		t.Errorf("Expected to peek item1, got different item")
	}

	// Test Update
	newDeadline1 := mockPriorityQueue.Peek().Deadline.Add(40 * time.Minute)
	mockPriorityQueue.Update(mockPriorityQueue.Peek(), newDeadline1)
	if !mockPriorityQueue.Peek().Deadline.Equal(newDeadline1) {
		t.Log("Correctly updated deadline of item1, now item2 is the peek item")
	} else {
		t.Errorf("Expected to peek item2, but got item1")
	}

	newDeadline2 := mockPriorityQueue.Peek().Deadline.Add(40 * time.Minute)
	mockPriorityQueue.Update(mockPriorityQueue.Peek(), newDeadline2)

	// Test Pop
	popped := mockPriorityQueue.Pop()
	if popped != firstItem {
		t.Errorf("Expected to pop item1, got different item")
	}
	if mockPriorityQueue.Len() != 1 {
		t.Errorf("Expected length 1, got %d", mockPriorityQueue.Len())
	}

	// Test Remove
	mockPriorityQueue.Remove(mockPriorityQueue.Peek())
	if mockPriorityQueue.Len() != 0 {
		t.Errorf("Expected length 0, got %d", mockPriorityQueue.Len())
	}
}

func TestDeadlineImpl(t *testing.T) {
	mockDeadlineHeap := &nexusHeap{}

	// Test Len
	if mockDeadlineHeap.Len() != 0 {
		t.Errorf("Expected length 0, got %d", mockDeadlineHeap.Len())
	}

	// Test Push
	startTime := time.Now()

	mockItemList := []*common.NexusItem{
		{
			Index:    0,
			Deadline: startTime,
		},
		{
			Index:    1,
			Deadline: startTime.Add(20 * time.Second),
		},
	}
	firstItem := mockItemList[0]

	// Test Push
	for _, item := range mockItemList {
		mockDeadlineHeap.Push(item)
	}
	if mockDeadlineHeap.Len() != 2 {
		t.Errorf("Expected length 2, got %d", mockDeadlineHeap.Len())
	}

	// Test Less
	if !mockDeadlineHeap.Less(0, 1) {
		t.Errorf("Expected item1 to be less than item2")
	}

	// Test Swap
	mockDeadlineHeap.Swap(0, 1)
	if mockDeadlineHeap.Less(0, 1) {
		t.Errorf("Expected item2 to be less than item1 after swap")
	}

	// Test Pop
	popped := mockDeadlineHeap.Pop()
	if popped != firstItem {
		t.Errorf("Expected to pop item1, got different item")
	}
	if mockDeadlineHeap.Len() != 1 {
		t.Errorf("Expected length 1, got %d", mockDeadlineHeap.Len())
	}
}

// this test will test if the queue gives all items until the given deadline
func TestGetAllItemsUntilDeadline(t *testing.T) {

	mockPriorityQueue := Initialize()

	mockItemList := []*common.NexusItem{
		{
			Deadline: time.Now(),
			Name:     "Now",
		},
		{
			Deadline: time.Now().Add(21 * time.Second),
			Name:     "Future",
		},
		{
			Deadline: time.Now(),
			Name:     "Now",
		},
		{
			Deadline: time.Now().Add(22 * time.Second),
			Name:     "Future",
		},
		{
			Deadline: time.Now(),
			Name:     "Now",
		},
		{
			Deadline: time.Now().Add(23 * time.Second),
			Name:     "Future",
		},
	}

	// Test Push
	for _, item := range mockItemList {
		mockPriorityQueue.Push(item)
	}

	if mockPriorityQueue.Len() != 6 {
		t.Errorf("Expected length 3, got %d", mockPriorityQueue.Len())
	}

	items := mockPriorityQueue.GetAllItemsUntilDeadline(time.Now().Add(10 * time.Second))
	if len(items) != 3 {
		t.Errorf("Expected length 3, got %d", len(items))
	}

	for _, item := range items {
		if item.Name != "Now" {
			t.Errorf("Expected to get only items with name 'Now', got %s", item.Name)
		}
	}

	mockPriorityQueue.RemoveAll(items)

	for _, item := range *mockPriorityQueue.impl {
		if item.Name == "Now" {
			t.Errorf("Expected to remove all Now items, but there are still %s left", item.Name)
		}
	}

	items = mockPriorityQueue.GetAllItemsUntilDeadline(time.Now().Add(30 * time.Second))
	if len(items) != 3 {
		t.Errorf("Expected length 3, got %d", len(items))
	}

	if mockPriorityQueue.Len() != 3 {
		t.Errorf("Expected length 3, got %d", mockPriorityQueue.Len())
	}

	mockPriorityQueue.RemoveAll(items)

	if mockPriorityQueue.Len() != 0 {
		t.Errorf("Expected length 0, got %d", mockPriorityQueue.Len())
	}

	items = mockPriorityQueue.GetAllItemsUntilDeadline(time.Now().Add(30 * time.Second))
	if len(items) != 0 {
		t.Errorf("Expected length 0, got %d", len(items))
	}
}

// Test PopBulkUntilDeadline
func TestPopBulkUntilDeadline(t *testing.T) {
	mockPriorityQueue := Initialize()

	mockItemList := []*common.NexusItem{
		{
			Deadline: time.Now(),
			Name:     "Item1",
		},
		{
			Deadline: time.Now().Add(20 * time.Second),
			Name:     "Item2",
		},
		{
			Deadline: time.Now().Add(20 * time.Second),
			Name:     "Item3",
		},
	}

	// Test Push
	for _, item := range mockItemList {
		mockPriorityQueue.Push(item)
	}

	items := mockPriorityQueue.PopBulkUntilDeadline(time.Now().Add(30 * time.Second))
	if len(items) != 3 {
		t.Errorf("Expected length 3, got %d", len(items))
	}

	if mockPriorityQueue.Len() != 0 {
		t.Errorf("Expected length 0, got %d", mockPriorityQueue.Len())
	}

	items = mockPriorityQueue.PopBulkUntilDeadline(time.Now().Add(30 * time.Second))

	if len(items) != 0 {
		t.Errorf("Expected length 0, got %d", len(items))
	}
}

// this will test if the queue can handle an index change after Getting the most common entry indices
func TestGetMostCommonEntryItemsPushRemove(t *testing.T) {

	mockPriorityQueue := Initialize()

	mockItemList := []*common.NexusItem{
		{
			Deadline: time.Now(),
			Name:     "LessCommon",
		},
		{
			Deadline: time.Now().Add(20 * time.Second),
			Name:     "MostCommon",
		},
		{
			Deadline: time.Now().Add(20 * time.Second),
			Name:     "MostCommon",
		},
		{
			Deadline: time.Now().Add(20 * time.Second),
			Name:     "MostCommon",
		},
	}

	// Test Push
	for _, item := range mockItemList {
		mockPriorityQueue.Push(item)
	}

	if mockPriorityQueue.Len() != 4 {
		t.Errorf("Expected length 3, got %d", mockPriorityQueue.Len())
	}

	items := mockPriorityQueue.GetMostCommonEntryItems()
	if len(items) != 3 {
		t.Errorf("Expected length 2, got %d", len(items))
	}

	PushItem := &common.NexusItem{
		Deadline: time.Now(),
		Name:     "LessCommon",
	}
	mockPriorityQueue.Push(PushItem)

	mockPriorityQueue.RemoveAll(items)

	count := 0
	for _, item := range *mockPriorityQueue.impl {
		if item.Name == "MostCommon" {
			count++
		}
	}

	if count > 0 {
		t.Errorf("Expected to remove all MostCommon items, but there are still %d left", count)
	}
}
