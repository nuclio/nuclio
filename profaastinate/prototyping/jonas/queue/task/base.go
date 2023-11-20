package task

type Task struct {
	Name  string
	Score float64
}

func NewTask(name string, score float64) *Task {
	return &Task{name, score}
}
