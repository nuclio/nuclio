package main

import (
	"fmt"
	"github.com/go-co-op/gocron"
	"time"
)

func main() {
	// Create a new scheduler
	s := gocron.NewScheduler(time.UTC)

	// Schedule the task function to run every second
	s.Every(1).Second().Do(taskFunction)

	// Start the scheduler asynchronously
	s.StartAsync()

	// Run the scheduler for 1 minute
	time.Sleep(1 * time.Minute)

	// Stop the scheduler after 1 minute
	s.Stop()
}

func taskFunction() {
	fmt.Println("Executing task every second")
}
