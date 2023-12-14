package common

import (
	common "nexus/common/models/structs"
	"testing"
	"time"
)

func TestPriorityQueue(t *testing.T) {
	mockPriorityQueue := Init()

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
	mockDeadlineHeap := &deadlineHeap{}

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

// this will test if the queue can handle a index change after Geting the most common entry indices
func TestGetMostCommonEntryItemsPushRemove(t *testing.T) {

	mockPriorityQueue := Init()

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
