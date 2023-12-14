package structs

type BaseNexusItem struct {
	Value interface{} // The value of the NexusEntry; arbitrary.
	Index int         // The index of the NexusEntry in the heap.
	ID    string
}

type NexusBaseItemImpl[C BaseNexusItem] struct {
	Queue []*C
}
