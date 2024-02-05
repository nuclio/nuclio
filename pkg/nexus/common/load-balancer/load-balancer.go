package load_balancer

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
)

// The LoadBalancer is responsible for balancing the load between the different function containers
// It is responsible for setting the maxParallelRequests for the different schedulers
// The schedulers tell the balancer how many functions they executed, and the balancer calculates a system load and
// tries to align it with the target load
//
// For more information see profaastinate/docs/diagrams/uml/activity/load-balancer-schedule.puml
type LoadBalancer struct {
	// The maximum number of parallel requests that can be executed
	maxParallelRequests *atomic.Int32
	// The highest number maxParallelRequests can be set to
	limitParallelRequests int
	// The flag that indicates if the LoadBalancer is running
	runningFlag bool
	// The time between two load calculations, used to avoid thrashing
	slidingWindowsDuration time.Duration
	// The channel that contains the function names that are executed
	functionExecutionChannel chan string
	// The target load for the CPU
	targetLoadCPU float64
	// The target load for the Memory
	targetLoadMemory float64
}

// NewLoadBalancer creates a new LoadBalancer
func NewLoadBalancer(maxParallelRequests *atomic.Int32, executionChannel chan string, collectionTime time.Duration, targetLoadCPU, targetLoadMemory float64) *LoadBalancer {
	return &LoadBalancer{
		maxParallelRequests:      maxParallelRequests,
		functionExecutionChannel: executionChannel,
		runningFlag:              false,
		targetLoadCPU:            targetLoadCPU,
		targetLoadMemory:         targetLoadMemory,
		slidingWindowsDuration:   collectionTime,
	}
}

// NewDefaultLoadBalancer creates a new LoadBalancer with default values
// The default values are:
// slidingWindowsDuration: 1 minute
// targetLoadCPU: 0
// targetLoadMemory: 0
func NewDefaultLoadBalancer(maxParallelRequests *atomic.Int32, executionChannel chan string) *LoadBalancer {
	return NewLoadBalancer(maxParallelRequests, executionChannel, 1*time.Minute, 0, 0)
}

// Initialize initializes the LoadBalancer after creation
func (lb *LoadBalancer) Initialize() {
}

// Start starts the LoadBalancer after initialization
func (lb *LoadBalancer) Start() {
	lb.runningFlag = true

	for lb.runningFlag {
		lb.AutoBalance()

		time.Sleep(lb.slidingWindowsDuration)
	}
}

// Stop stops the LoadBalancer
func (lb *LoadBalancer) Stop() {
	lb.runningFlag = false
}

// SetTargetLoadCPU sets the target load for the CPU
func (lb *LoadBalancer) SetTargetLoadCPU(targetLoadCPU float64) {
	lb.targetLoadCPU = targetLoadCPU
}

// SetTargetLoadMemory sets the target load for the Memory
func (lb *LoadBalancer) SetTargetLoadMemory(targetLoadMemory float64) {
	lb.targetLoadMemory = targetLoadMemory
}

// SetLimitParallelRequests sets the highest number maxParallelRequests can be set to
func (lb *LoadBalancer) SetLimitParallelRequests(limitParallelRequests int) {
	lb.limitParallelRequests = limitParallelRequests
}

// cpuMock and memMock are interfaces for mocking the CPU and Memory information in the tests
var cpuMock = cpu.Percent
var memMock = mem.VirtualMemory

// CalculateDesiredNumberOfRequestsCPU calculates the desired number of requests based on the CPU load
func (lb *LoadBalancer) CalculateDesiredNumberOfRequestsCPU(numberOfExecutedFunctionCalls int) int {
	cpuLoadPercentageInfo, err := cpuMock(lb.slidingWindowsDuration, true)
	if err != nil {
		fmt.Print("Error retrieving CPU information:", err)
	} else if len(cpuLoadPercentageInfo) == 0 {
		fmt.Print("Error retrieving CPU information: no CPU information available")
	}

	avgPercentage := 0.0
	for _, percentage := range cpuLoadPercentageInfo {
		avgPercentage += percentage
	}
	avgPercentage /= float64(len(cpuLoadPercentageInfo))

	return int(float64(numberOfExecutedFunctionCalls) * (lb.targetLoadCPU / avgPercentage))
}

// CalculateDesiredNumberOfRequestsMemory calculates the desired number of requests based on the Memory load
func (lb *LoadBalancer) CalculateDesiredNumberOfRequestsMemory(numberOfExecutedFunctionCalls int) int {
	virtualMemory, err := memMock()
	if err != nil {
		fmt.Print("Error retrieving Memory information:", err)
	}

	return int(float64(numberOfExecutedFunctionCalls) * (lb.targetLoadMemory / virtualMemory.UsedPercent))
}

// AutoBalance tries to balance the load between the different function containers
// It tries to align the system load with the target load for the CPU and Memory
// For more information see profaastinate/docs/diagrams/uml/activity/load-balancer-schedule.puml
func (lb *LoadBalancer) AutoBalance() {
	fmt.Printf("AutoBalancing")

	executedFunctionMap := make(map[string]int)
	numberOfExecutedFunctionCalls := 0
	for {
		select {
		case executedFunction, ok := <-lb.functionExecutionChannel:
			if !ok {
				// Used after no item is left in the channel

				return
			}
			executedFunctionMap[executedFunction]++
			numberOfExecutedFunctionCalls++

		default:
			fmt.Printf("No item in channel")
			fmt.Print(executedFunctionMap)

			// Used when the channel is empty
			if numberOfExecutedFunctionCalls == 0 {
				return
			}

			var avgDesiredNumber int

			switch {
			case lb.targetLoadCPU == 0 && lb.targetLoadMemory == 0:
				return
			case lb.targetLoadCPU == 0:
				avgDesiredNumber = lb.CalculateDesiredNumberOfRequestsMemory(numberOfExecutedFunctionCalls)
			case lb.targetLoadMemory == 0:
				avgDesiredNumber = lb.CalculateDesiredNumberOfRequestsCPU(numberOfExecutedFunctionCalls)
			default:
				desiredNumberMemory := lb.CalculateDesiredNumberOfRequestsMemory(numberOfExecutedFunctionCalls)
				desiredNumberCPU := lb.CalculateDesiredNumberOfRequestsCPU(numberOfExecutedFunctionCalls)
				avgDesiredNumber = (desiredNumberMemory + desiredNumberCPU) / 2
			}

			if lb.limitParallelRequests > 0 && lb.limitParallelRequests < avgDesiredNumber {
				avgDesiredNumber = lb.limitParallelRequests
			}

			lb.maxParallelRequests.Store(int32(avgDesiredNumber))
			lb.functionExecutionChannel = make(chan string, avgDesiredNumber*10)
			fmt.Printf("The maxProcessingRequests was set to %d\n", avgDesiredNumber)
			return
		}
	}
}
