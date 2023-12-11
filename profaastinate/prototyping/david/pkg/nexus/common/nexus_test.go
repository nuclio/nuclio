package common

import (
	"github.com/google/uuid"
	"github.com/konsumgandalf/mpga-protoype-david/pkg/nexus/common/interfaces"
	"github.com/konsumgandalf/mpga-protoype-david/pkg/nexus/common/structs"
	"sync"
	"testing"
	"time"
)

type AbstractMockScheduler struct {
	nexus *Nexus
	Items []structs.BaseNexusItem
}

func (m *AbstractMockScheduler) Find(itemID string) (int, structs.BaseNexusItem) {
	for idx, item := range m.Items {
		if item.ID == itemID {
			return idx, item
		}
	}
	return -1, structs.BaseNexusItem{}
}

func (m *AbstractMockScheduler) Remove(itemID string) {
	idx, _ := m.Find(itemID)

	if idx != -1 {
		ret := make([]structs.BaseNexusItem, 0)
		ret = append(ret, m.Items[:idx]...)
		m.Items = append(ret, m.Items[idx+1:]...)
	}
}

func (m *AbstractMockScheduler) Push(item structs.BaseNexusItem) {
	m.Items = append(m.Items, item)
}

// Mock scheduler implementing INexusScheduler interface
type MockScheduler1 struct {
	AbstractMockScheduler
}

type MockScheduler2 struct {
	AbstractMockScheduler
}

func (m *MockScheduler1) Pop(itemID string) (firstItem structs.BaseNexusItem) {
	firstItem = m.Items[0]

	ret := make([]structs.BaseNexusItem, 0)
	m.Items = append(ret, m.Items[1:]...)
	m.nexus.CallbackRemove(*m, itemID)
	return
}

func (m *MockScheduler2) Pop(itemID string) (firstItem structs.BaseNexusItem) {
	firstItem = m.Items[0]

	m.Items = append(m.Items[:0], m.Items[1:]...)
	m.nexus.CallbackRemove(*m, itemID)
	return
}

type SlowScheduler struct {
	AbstractMockScheduler
}

func (m *SlowScheduler) Push(item structs.BaseNexusItem) {
	time.Sleep(100 * time.Millisecond)

	m.AbstractMockScheduler.Push(item)
}

func (m *SlowScheduler) Pop(itemID string) (firstItem structs.BaseNexusItem) {
	time.Sleep(100 * time.Millisecond)

	firstItem = m.Items[0]

	m.Items = append(m.Items[:0], m.Items[1:]...)
	m.nexus.CallbackRemove(*m, itemID)
	return
}

func SetUp(t *testing.T) (*Nexus, *MockScheduler1, *MockScheduler2, *SlowScheduler, []structs.BaseNexusItem) {
	nexus := &Nexus{
		nexusScheduler: make(map[string]interfaces.INexusScheduler),
		mu:             &sync.RWMutex{},
	}

	mockScheduler1 := &MockScheduler1{
		AbstractMockScheduler{
			nexus: nexus,
		},
	}
	mockScheduler2 := &MockScheduler2{
		AbstractMockScheduler{
			nexus: nexus,
		},
	}
	slowScheduler := &SlowScheduler{
		AbstractMockScheduler{
			nexus: nexus,
		},
	}

	id1, id2, id3 := uuid.NewString(), uuid.NewString(), uuid.NewString()

	items := []structs.BaseNexusItem{
		structs.BaseNexusItem{
			Value: "test1",
			Index: 0,
			ID:    id1,
		},
		structs.BaseNexusItem{
			Value: "test1",
			Index: 1,
			ID:    id2,
		},
		structs.BaseNexusItem{
			Value: "test2",
			Index: 2,
			ID:    id3,
		},
	}

	mockScheduler1.Items = items
	mockScheduler2.Items = items
	slowScheduler.Items = items

	// Add mock schedulers to the Nexus
	nexus.nexusScheduler["MockScheduler1"] = mockScheduler1
	nexus.nexusScheduler["MockScheduler2"] = mockScheduler2
	nexus.nexusScheduler["SlowScheduler"] = slowScheduler

	return nexus, mockScheduler1, mockScheduler2, slowScheduler, items
}

func TestNexusPop(t *testing.T) {
	nexus, mockScheduler1, mockScheduler2, slowScheduler, items := SetUp(t)

	deletedItemID := items[0].ID

	deletedItem := nexus.nexusScheduler["MockScheduler1"].Pop(deletedItemID)

	if len(mockScheduler1.Items) != 2 {
		t.Fatalf("Expected scheduler1 to have 1 item, but got %d", len(mockScheduler1.Items))
	}

	if len(slowScheduler.Items) != 2 {
		t.Fatalf("Expected slowScheduler to have 1 item, but got %d", len(slowScheduler.Items))
	}

	if len(mockScheduler2.Items) != 2 {
		t.Errorf("Expected scheduler2 to have 1 item, but got %d", len(mockScheduler2.Items))
	}

	if deletedItem.ID != deletedItemID {
		t.Fatalf("Unexpected item ID returned from Pop. Expected %s, got %s", deletedItemID, deletedItem.ID)
	}
}

func TestNexusPush(t *testing.T) {
	nexus, mockScheduler1, mockScheduler2, slowScheduler, _ := SetUp(t)

	id4, numberOfElement := uuid.NewString(), 4

	nexus.Push(structs.BaseNexusItem{
		Value: "test4",
		Index: numberOfElement - 1,
		ID:    id4,
	})

	if len(mockScheduler1.Items) != numberOfElement {
		t.Fatalf("Expected scheduler1 to have 3 items, but got %d", len(mockScheduler1.Items))
	}

	if len(mockScheduler2.Items) != numberOfElement {
		t.Fatalf("Expected scheduler2 to have 3 items, but got %d", len(mockScheduler2.Items))
	}

	if len(slowScheduler.Items) != numberOfElement {
		t.Fatalf("Expected slowScheduler to have 3 items, but got %d", len(mockScheduler2.Items))
	}
}
