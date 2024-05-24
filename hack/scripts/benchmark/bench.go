package main

import (
	"flag"
	"fmt"
	"net/http"
	"sync"
	"time"
)

func main() {
	// define command-line arguments
	url := flag.String("url", "", "The URL to send requests to")
	totalRequests := flag.Int("totalRequests", 100, "Total number of requests to send")
	maxParallel := flag.Int("maxParallel", 1000, "Maximum number of parallel requests")

	// parse the command-line arguments
	flag.Parse()

	// waitGroup to synchronize completion of goroutines
	var wg sync.WaitGroup

	// channel to receive timing results from goroutines
	results := make(chan time.Duration, *totalRequests)

	// semaphore to limit the number of concurrent goroutines
	sem := make(chan struct{}, *maxParallel)

	// iterate over the total number of requests and spawn goroutines to send requests concurrently
	for i := 0; i < *totalRequests; i++ {
		wg.Add(1)
		sem <- struct{}{} // acquire semaphore slot
		go func() {
			defer wg.Done()
			defer func() { <-sem }() // release semaphore slot

			start := time.Now()
			resp, err := http.Get(*url)
			if err != nil {
				fmt.Printf("Error sending request: %v\n", err)
				return
			}
			defer resp.Body.Close()

			elapsed := time.Since(start)
			results <- elapsed
		}()
	}

	// wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// collect timing results from the channel and calculate statistics
	totalTime := time.Duration(0)
	numCompleted := 0

	for elapsed := range results {
		totalTime += elapsed
		numCompleted++
		fmt.Printf("Request took: %v\n", elapsed)
	}

	// calculate average response time
	if numCompleted > 0 {
		avgTime := totalTime / time.Duration(numCompleted)
		fmt.Printf("Average response time: %v\n", avgTime)
	} else {
		fmt.Println("No requests were successful")
	}
}
