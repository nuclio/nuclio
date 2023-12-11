package tree

import (
	"fmt"
	"strings"

	"github.com/Persists/profaastinate-queue/task"
)

// Tree struct with Root node
type Tree struct {
	Root *Node
}

// Node struct with Left and Right nodes, Height, Deadline and Task
type Node struct {
	Left     *Node
	Right    *Node
	Height   int
	Deadline int64
	Tasks    []*task.Task
}

// Function to get the height of a node
func (n *Node) height() int {
	if n == nil {
		return -1
	}
	return n.Height
}

// Function to update the height of a node
func (n *Node) updateHeight() {
	leftHeight := -1
	if n.Left != nil {
		leftHeight = n.Left.height()
	}
	rightHeight := -1
	if n.Right != nil {
		rightHeight = n.Right.height()
	}
	if leftHeight > rightHeight {
		n.Height = leftHeight + 1
	} else {
		n.Height = rightHeight + 1
	}
}

// Function to get the balance factor of a node
func (n *Node) balanceFactor() int {
	leftHeight := -1
	if n.Left != nil {
		leftHeight = n.Left.height()
	}
	rightHeight := -1
	if n.Right != nil {
		rightHeight = n.Right.height()
	}
	return leftHeight - rightHeight
}

// Function to rotate a node to the left
func (n *Node) rotateLeft() *Node {
	newRoot := n.Right
	n.Right = newRoot.Left
	newRoot.Left = n
	n.updateHeight()
	newRoot.updateHeight()
	return newRoot
}

// Function to rotate a node to the right
func (n *Node) rotateRight() *Node {
	newRoot := n.Left
	n.Left = newRoot.Right
	newRoot.Right = n
	n.updateHeight()
	newRoot.updateHeight()
	return newRoot
}

// Function to balance a node
func (n *Node) balance() *Node {
	balanceFactor := n.balanceFactor()
	if balanceFactor > 1 {
		if n.Left.balanceFactor() < 0 {
			n.Left = n.Left.rotateLeft()
		}
		return n.rotateRight()
	} else if balanceFactor < -1 {
		if n.Right.balanceFactor() > 0 {
			n.Right = n.Right.rotateRight()
		}
		return n.rotateLeft()
	}
	return n
}

// Function to insert a task into the tree
func (t *Tree) Insert(tsk *task.Task) {
	if t.Root == nil {
		t.Root = &Node{
			Deadline: tsk.Deadline.Unix(),
			Tasks:    []*task.Task{tsk},
		}
	} else {
		t.Root = t.Root.insert(tsk, tsk.Deadline.Unix())
	}
}

// Function to insert a task into a node
func (n *Node) insert(tsk *task.Task, deadline int64) *Node {
	if n == nil {
		return &Node{
			Deadline: deadline,
			Tasks:    []*task.Task{tsk},
		}
	}
	if deadline < n.Deadline {
		n.Left = n.Left.insert(tsk, deadline)
	} else if deadline > n.Deadline {
		n.Right = n.Right.insert(tsk, deadline)
	} else {
		n.Tasks = append(n.Tasks, tsk)
	}
	n.updateHeight()
	return n.balance()
}

// Function to pop the tasks with the lowest deadline from the tree
func (t *Tree) PopLowestDeadline(amount int) []*task.Task {
	if t.Root == nil {
		return nil
	}
	tasks, n := t.Root.popLowestDeadline(amount)
	t.Root = n

	return tasks
}

// Function to pop the tasks with the lowest deadline from a node
func (n *Node) popLowestDeadline(amount int) ([]*task.Task, *Node) {
	if n == nil {
		return nil, nil
	}

	var poppedTasks []*task.Task

	// if Left is not nil, pop from Left first because it has lower deadlines
	if n.Left != nil {
		// pop from Left
		// it returns the popped tasks and the new Left rebalanced node
		tasks, left := n.Left.popLowestDeadline(amount)
		poppedTasks = append(poppedTasks, tasks...)
		n.Left = left

		// if the popped amount is reached return the popped tasks and the rebalanced node
		if len(poppedTasks) >= amount {
			n.updateHeight()
			return poppedTasks, n.balance()
		}
	}

	// if the popped amount is not reached, pop from the current node
	if len(poppedTasks) < amount {

		// if the popped amount is reached, cut the tasks to the amount needed
		if len(n.Tasks) > amount-len(poppedTasks) {

			// cut the tasks to the amount needed
			cutIndex := amount - len(poppedTasks)
			poppedTasks = append(poppedTasks, n.Tasks[:cutIndex]...)

			// remove the popped tasks from the node
			n.Tasks = n.Tasks[cutIndex:]

			// else pop all the tasks from the node and delete the node
		} else {
			poppedTasks = append(poppedTasks, n.Tasks...)
			n.Tasks = nil
		}
	}

	// if the popped amount is not reached, pop from Right
	if len(poppedTasks) < amount && n.Right != nil {

		// pop from Right
		// it returns the popped tasks and the new Right rebalanced node
		tasks, right := n.Right.popLowestDeadline(amount - len(poppedTasks))
		poppedTasks = append(poppedTasks, tasks...)
		n.Right = right
	}

	// if tasks are empty, delete the node
	// if Right is not nil, replace the node with Right
	if n.Tasks == nil || len(n.Tasks) == 0 {
		if n.Right != nil {
			n = n.Right
		} else {
			return poppedTasks, nil
		}
	}

	// update the height and return the popped tasks and the rebalanced node
	n.updateHeight()
	return poppedTasks, n.balance()
}

func (n *Node) RemoveTask(tsk *task.Task, deadline int64) *Node {
	if n == nil {
		return nil
	}

	if n.Deadline == deadline {
		for i, t := range n.Tasks {
			if t == tsk {
				n.Tasks[i] = n.Tasks[len(n.Tasks)-1]
				n.Tasks = n.Tasks[:len(n.Tasks)-1]
				break
			}
		}
	} else if n.Deadline < deadline {
		n.Right = n.Right.RemoveTask(tsk, deadline)
	} else {
		n.Left = n.Left.RemoveTask(tsk, deadline)
	}

	if len(n.Tasks) == 0 {
		if n.Left != nil {
			n = n.Left
		} else {
			n = n.Right
		}
	}

	if n != nil {
		n.updateHeight()
		n = n.balance()
	}

	return n
}

// Function to remove a task from the tree
func (t *Tree) RemoveTask(tsk *task.Task) {
	if t.Root != nil {
		t.Root = t.Root.RemoveTask(tsk, tsk.Deadline.Unix())
	}
}

// Function to pop the tasks with the lowest deadline from a node

// Function to traverse a node from low to high
func (n *Node) traverseLowToHighChannel(c chan *task.Task) {

	if n.Left != nil {
		n.Left.traverseLowToHighChannel(c)
	}
	for _, t := range n.Tasks {
		c <- t
	}
	if n.Right != nil {
		n.Right.traverseLowToHighChannel(c)
	}
}

// Function to traverse the tree from low to high
func (t *Tree) TraverseLowToHighChannel(c chan *task.Task) {
	if t.Root == nil {
		return
	}
	t.Root.traverseLowToHighChannel(c)
}

// Function to traverse a node from low to high and return tasks
func (n *Node) traversalLowToHigh() []*task.Task {
	var tasks []*task.Task
	if n.Left != nil {
		tasks = append(tasks, n.Left.traversalLowToHigh()...)
	}
	tasks = append(tasks, n.Tasks...)
	if n.Right != nil {
		tasks = append(tasks, n.Right.traversalLowToHigh()...)
	}
	return tasks
}

// Function to traverse the tree from low to high and return tasks
func (t *Tree) TraversalLowToHigh() []*task.Task {
	if t.Root == nil {
		return nil
	}
	return t.Root.traversalLowToHigh()
}

// Function to find a task in a node
func (n *Node) find(find func(*task.Task) bool) *task.Task {
	for _, tsk := range n.Tasks {
		if find(tsk) {
			return tsk
		}
	}

	if n.Left != nil {
		tsk := n.Left.find(find)
		if tsk != nil {
			return tsk
		}
	}

	if n.Right != nil {
		tsk := n.Right.find(find)
		if tsk != nil {
			return tsk
		}
	}

	return nil
}

// Function to find a task in the tree
// & doesnt remove the task
func (t *Tree) Find(find func(*task.Task) bool) *task.Task {
	if t.Root == nil {
		return nil
	}
	return t.Root.find(find)
}

// Function to filter tasks in a node
// & doesnt remove the tasks
func (n *Node) filter(filter func(*task.Task) bool) []*task.Task {
	var tasks []*task.Task
	for _, tsk := range n.Tasks {
		if filter(tsk) {
			tasks = append(tasks, tsk)
		}
	}

	if n.Left != nil {
		tasks = append(tasks, n.Left.filter(filter)...)
	}

	if n.Right != nil {
		tasks = append(tasks, n.Right.filter(filter)...)
	}

	return tasks
}

// Function to filter tasks in the tree
func (t *Tree) Filter(filter func(*task.Task) bool) []*task.Task {
	if t.Root == nil {
		return nil
	}
	return t.Root.filter(filter)
}

// Function to print the tree structure
func (t *Tree) PrintTreeStructure() {
	if t.Root == nil {
		fmt.Println("Empty tree")
		return
	}
	println("Tree structure:")
	t.Root.printTreeStructure("ROOT", 0)
}

// Function to print a node structure
func (n *Node) printTreeStructure(position string, level int) {
	if n.Right != nil {
		n.Right.printTreeStructure("R", level+1)
	}
	println(len(n.Tasks))
	fmt.Println(strings.Repeat("    ", level), position, "lvl-", level, "-", n.Tasks[0].Name, "len:", len(n.Tasks))
	if n.Left != nil {
		n.Left.printTreeStructure("L", level+1)
	}
}

func InitTree() *Tree {
	return &Tree{}
}
