// Golang program to illustrate the usage of
// Sleep() function

// Including main package
package main

// Importing fmt and time
import (
	"fmt"
	"time"
)

// Main function
func main() {

	for true {

		// Calling Sleep method
		time.Sleep(2 * time.Second)
		fmt.Println("Sleep Over.....")
		continue
		// Printed after sleep is over
	}
}
