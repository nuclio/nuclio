package load_balancer

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/shirou/gopsutil/mem"
	"github.com/stretchr/testify/suite"
)

type LoadBalancerTestSuite struct {
	suite.Suite
	loadBalancer *LoadBalancer
}

func (suite *LoadBalancerTestSuite) populateChannel() {
	suite.loadBalancer.functionExecutionChannel <- "test-hello-world"
	suite.loadBalancer.functionExecutionChannel <- "test-hello-world"
	suite.loadBalancer.functionExecutionChannel <- "test-bye-world"
	suite.loadBalancer.functionExecutionChannel <- "test-bye-world"
}

func (suite *LoadBalancerTestSuite) SetupTest() {
	cpuMock = func(interval time.Duration, percpu bool) ([]float64, error) {
		return []float64{10.0, 20.0, 30.0}, nil
	}
	memMock = func() (*mem.VirtualMemoryStat, error) {
		return &mem.VirtualMemoryStat{
			UsedPercent: 24.5,
		}, nil
	}

	var maxParallelRequests atomic.Int32
	maxParallelRequests.Store(200)
	executionChannel := make(chan string, maxParallelRequests.Load()*10)

	suite.loadBalancer = NewLoadBalancer(
		&maxParallelRequests,
		executionChannel,
		1*time.Second,
		50.0,
		50.0,
	)
}

func (suite *LoadBalancerTestSuite) TestCalculateDesiredNumberOfRequestsCPU() {
	desiredNumberOfRequests := suite.loadBalancer.CalculateDesiredNumberOfRequestsCPU(4)

	// Current Load 20.0, TargetLoad 50.0, CurrentNumberOfRequests 4 => DesiredNumberOfRequests 50 / 20 * 4 = 10
	suite.Equal(10, desiredNumberOfRequests)
}

func (suite *LoadBalancerTestSuite) TestCalculateDesiredNumberOfRequestsMemory() {
	desiredNumberOfRequests := suite.loadBalancer.CalculateDesiredNumberOfRequestsMemory(4)

	// Current Load 24.5, TargetLoad 50.0, CurrentNumberOfRequests 4 => DesiredNumberOfRequests 50 / 24.5 * 4 = 8
	suite.Equal(8, desiredNumberOfRequests)
}

func (suite *LoadBalancerTestSuite) TestSetTargetLoadCPU() {
	suite.loadBalancer.SetTargetLoadCPU(60.0)
	suite.Equal(60.0, suite.loadBalancer.targetLoadCPU)
}

func (suite *LoadBalancerTestSuite) TestSetTargetLoadMemory() {
	suite.loadBalancer.SetTargetLoadMemory(60.0)
	suite.Equal(60.0, suite.loadBalancer.targetLoadMemory)
}

func (suite *LoadBalancerTestSuite) TestAutoBalance() {
	suite.populateChannel()

	suite.loadBalancer.AutoBalance()

	// DesiredNumberCPU 10, DesiredNumberMemory 8 => DesiredNumberOfRequests 9
	suite.Equal(int32(9), suite.loadBalancer.maxParallelRequests.Load())

	suite.loadBalancer.AutoBalance()
	// No new values => Value is not adjusted
	suite.Equal(int32(9), suite.loadBalancer.maxParallelRequests.Load())

	suite.loadBalancer.targetLoadMemory = 0
	suite.populateChannel()
	suite.loadBalancer.AutoBalance()

	// DesiredNumberCPU 10, mem now ignored
	suite.Equal(int32(10), suite.loadBalancer.maxParallelRequests.Load())

}

func TestLoadBalancerTestSuite(t *testing.T) {
	suite.Run(t, new(LoadBalancerTestSuite))
}
