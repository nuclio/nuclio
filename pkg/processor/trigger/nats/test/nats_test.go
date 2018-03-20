package test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"testing"
	"time"

	common "github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/test/suite"

	"github.com/nats-io/go-nats"
	"github.com/stretchr/testify/suite"
)

const (
	natsPort  = 4222
	topicName = "my.topic"
)

type NatsTestSuite struct {
	processorsuite.TestSuite
	natsContainerId  string
	natsSubscription *nats.Subscription
	natsPort         int
	topicName        string
}

func (suite *NatsTestSuite) SetupSuite() {
	var err error

	suite.TestSuite.SetupSuite()

	suite.natsPort = natsPort
	suite.topicName = topicName

	// Start nats
	suite.natsContainerId, err = suite.DockerClient.RunContainer("nats:latest",
		&dockerclient.RunOptions{
			Ports: map[int]int{suite.natsPort: suite.natsPort, 8222: 8222},
		})

	suite.Require().NoError(err, "Failed to start NATS container")
}

func (suite *NatsTestSuite) SetupTest() {
	suite.TestSuite.SetupTest()

	suite.FunctionDir = suite.getFunctionsPath()
}

func (suite *NatsTestSuite) TearDownSuite() {
	suite.TestSuite.TearDownTest()

	// If we weren't successful starting, nothing to do
	if suite.natsContainerId != "" {
		suite.DockerClient.RemoveContainer(suite.natsContainerId)
	}
}

func (suite *NatsTestSuite) TestPostEventPython() {
	suite.invokeEventRecorder("event_recorder_python", "python")
}

func (suite *NatsTestSuite) TestPostEventGolang() {
	suite.invokeEventRecorder(path.Join("_event_recorder_golang", "event_recorder.go"), "golang")
}

func (suite *NatsTestSuite) invokeEventRecorder(functionPath string, runtimeType string) {
	suite.Runtime = runtimeType

	deployOptions := suite.GetDeployOptions("event_recorder",
		suite.GetFunctionPath(functionPath))

	suite.DeployFunction(deployOptions, func(deployResult *platform.DeployResult) bool {

		var natsConn *nats.Conn

		// Try to perform connection to Nats
		err := common.RetryUntilSuccessful(15*time.Second, 1*time.Second, func() bool {
			natsConn, _ = nats.Connect(nats.DefaultURL)

			// If we're connected to the Nats get up from the function
			if natsConn.IsConnected() {
				return true
			} else {

				return false
			}

		})

		// Verify that there's not error during Nats connection
		suite.Require().NoError(err, "Can't connect to NATS server %s", nats.DefaultURL)

		// Send 3 messages
		for requestIdx := 0; requestIdx < 3; requestIdx++ {
			err := natsConn.Publish(suite.topicName, []byte(fmt.Sprintf(`{"request": "%d"}`, requestIdx)))

			if err != nil {
				errors.Wrapf(err, "Can't sent message to NATS server")
				return false
			}

			// Add wait to be sure that messages successfully received
			time.Sleep(1 * time.Second)
		}

		natsConn.Close()

		baseURL := "localhost"

		// Set the url for the http request
		url := fmt.Sprintf("http://%s:%d", baseURL, deployResult.Port)

		// Read the events from the function
		httpResponse, err := http.Get(url)
		suite.Require().NoError(err, "Failed to read events from function: %s", url)

		marshalledResponseBody, err := ioutil.ReadAll(httpResponse.Body)
		suite.Require().NoError(err, "Failed to read response body")

		// Unmarshall the body into a list
		var receivedEvents []map[string]string

		err = json.Unmarshal(marshalledResponseBody, &receivedEvents)
		suite.Require().NoError(err, "Failed to unmarshal response")

		// Must have received 3 events
		suite.Require().Equal([]map[string]string{
			{"request": "0"},
			{"request": "1"},
			{"request": "2"},
		}, receivedEvents)

		return true
	})

}

func (suite *NatsTestSuite) getFunctionsPath() string {
	return path.Join(suite.GetNuclioSourceDir(), "pkg", "processor", "trigger", "nats", "test")
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(NatsTestSuite))
}
