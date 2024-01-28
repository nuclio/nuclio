package main

import (
	"fmt"
	"os"
	"time"

	"github.com/mackerelio/go-osstat/memory"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
)

func main() {
	memory, err := memory.Get()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return
	}
	fmt.Printf("memory total: %d bytes\n", memory.Total)
	fmt.Printf("memory used: %d bytes\n", memory.Used)
	fmt.Printf("memory cached: %d bytes\n", memory.Cached)
	fmt.Printf("memory free: %d bytes\n", memory.Free)

	vm, _ := mem.VirtualMemory()
	if err != nil {
		fmt.Println("Error retrieving memory information:", err)
		return
	}

	fmt.Printf("Total: %v, Free: %v, UsedPercent: %.2f%%\n", vm.Total, vm.Free, vm.UsedPercent)

	cpuPercentages, err2 := cpu.Percent(time.Second, true)
	if err2 != nil {
		fmt.Println("Error retrieving CPU information:", err)
		return
	}

	// Print the CPU usage percentages
	avgPercentage := 0
	for i, percentage := range cpuPercentages {
		avgPercentage += int(percentage)
		fmt.Printf("CPU%d: %.2f%%\n", i, percentage)
	}

	fmt.Println(cpuPercentages[0])
	fmt.Println(avgPercentage / len(cpuPercentages))
}
