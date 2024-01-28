package load_balancer

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
)

type LoadBalancer struct {
	maxParallelRequests      *atomic.Int32
	runningFlag              bool
	collectionTime           time.Duration
	functionExecutionChannel chan string // channel contains the function name that is executed
	targetLoadCPU            float64
	targetLoadMemory         float64
}

func NewLoadBalancer(maxParallelRequests *atomic.Int32, executionChannel chan string, collectionTime time.Duration, targetLoadCPU, targetLoadMemory float64) *LoadBalancer {
	return &LoadBalancer{
		maxParallelRequests:      maxParallelRequests,
		functionExecutionChannel: executionChannel,
		runningFlag:              false,
		targetLoadCPU:            targetLoadCPU,
		targetLoadMemory:         targetLoadMemory,
		collectionTime:           collectionTime,
	}
}

func NewDefaultLoadBalancer(maxParallelRequests *atomic.Int32, executionChannel chan string) *LoadBalancer {
	return NewLoadBalancer(maxParallelRequests, executionChannel, 1*time.Minute, 0, 0)
}

func (lb *LoadBalancer) Initialize() {
}

func (lb *LoadBalancer) Start() {
	lb.runningFlag = true

	for lb.runningFlag {
		lb.AutoBalance()

		time.Sleep(lb.collectionTime)
	}
}

func (lb *LoadBalancer) Stop() {
	lb.runningFlag = false
}

func (lb *LoadBalancer) SetTargetLoadCPU(targetLoadCPU float64) {
	lb.targetLoadCPU = targetLoadCPU
}

func (lb *LoadBalancer) SetTargetLoadMemory(targetLoadMemory float64) {
	lb.targetLoadMemory = targetLoadMemory
}

var cpuMock = cpu.Percent

func (lb *LoadBalancer) CalculateDesiredNumberOfRequestsCPU(numberOfExecutedFunctionCalls int) int {
	cpuLoadPercentageInfo, err := cpuMock(lb.collectionTime, true)
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

	cpuLoadPercentagePerFunctionCall := avgPercentage / float64(numberOfExecutedFunctionCalls)
	return int(lb.targetLoadCPU / cpuLoadPercentagePerFunctionCall)
}

var memMock = mem.VirtualMemory

func (lb *LoadBalancer) CalculateDesiredNumberOfRequestsMemory(numberOfExecutedFunctionCalls int) int {
	virtualMemory, err := memMock()
	if err != nil {
		fmt.Print("Error retrieving Memory information:", err)
	}

	memoryLoadPercentagePerFunctionCall := virtualMemory.UsedPercent / float64(numberOfExecutedFunctionCalls)
	return int(lb.targetLoadMemory / memoryLoadPercentagePerFunctionCall)
}

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
			lb.maxParallelRequests.Store(int32(avgDesiredNumber))
			fmt.Printf("The maxProcessingRequests was set to %d\n", avgDesiredNumber)
			return
		}
	}
}
