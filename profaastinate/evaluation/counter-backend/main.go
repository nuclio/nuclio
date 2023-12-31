package main

import (
	"fmt"
	"net/http"
	"sync"
)

const (
	EVALUATION_INVOCATION = "/evaluation/invocation"
	EVALUATION_HEADERS    = "/evaluation/headers"
)

// Counter struct holds the count and a mutex to ensure safe access
type Counter struct {
	count   int
	headers []http.Header
	mu      sync.RWMutex
}

func (c *Counter) handleFunctionInvocations(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		c.mu.Lock()
		defer c.mu.Unlock()

		// Increment count
		c.count++

		// Save headers
		c.headers = append(c.headers, r.Header)

		fmt.Fprintf(w, "Count increased by 1, Current Count: %d", c.count)
	case http.MethodGet:
		c.mu.RLock()
		defer c.mu.RUnlock()
		fmt.Fprintf(w, "Current Count: %d", c.count)
	case http.MethodDelete:
		c.mu.Lock()
		defer c.mu.Unlock()

		// Reset count and headers
		c.count = 0
		c.headers = nil

		fmt.Fprintf(w, "Count reset to: %d", c.count)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (c *Counter) handleFunctionHeaders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		c.mu.RLock()
		defer c.mu.RUnlock()

		fmt.Fprintf(w, "Headers: %+v", c.headers)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func main() {
	counter := &Counter{count: 0, headers: make([]http.Header, 0)}

	// Set up HTTP server with two endpoints
	http.HandleFunc(EVALUATION_INVOCATION, counter.handleFunctionInvocations)
	http.HandleFunc(EVALUATION_HEADERS, counter.handleFunctionHeaders)

	// Start the server
	fmt.Println("Server listening on :8888")
	http.ListenAndServe(":8888", nil)
}
