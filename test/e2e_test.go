/*
End to end test

We do the following steps:
- Clearn k8s from previous deployment
- Build and deploy a controller
- Generate unique ID for this test
- Build and deploy a HTTP function that returns the test ID in the response body
- Perform a GET on the handler and see we got the right response
*/
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/stretchr/testify/suite"

	nuclio "github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/util/cmdrunner"
	nucliozap "github.com/nuclio/nuclio/pkg/zap"
)

const (
	imageName    = "nuclio/controller"
	k8sHost      = "52.16.125.41"
	registryPort = 31276
	HTTPPort     = 31010
)

var options struct {
	local bool
}

var handlerTemplate = template.Must(template.New("handler").Parse(`
package handler

import (
    "github.com/nuclio/nuclio-sdk"
)

func Handler(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
    context.Logger.Info("Event received")

    return nuclio.Response{
        StatusCode:  200,
        ContentType: "application/text",
        Body: []byte("{{.}}"),
    }, nil
}
`))

var kubeTemplate = template.Must(template.New("kube").Parse(`
apiVersion: nuclio.io/v1
kind: Function
metadata:
  name: handler
spec:
  replicas: 1
  image: localhost:5000/{{.Tag}}
  httpPort: {{.Port}}
`))

type kubeParame struct {
	Tag  string
	Port int
}

func init() {
	flag.BoolVar(&options.local, "local", false, "get local copy of nuclio")
	flag.Parse()
}

// newTestID return new unique test ID
func newTestID() string {
	host, err := os.Hostname()
	if err != nil {
		host = "unkown-host"
	}

	return fmt.Sprintf("nuclio-e2e-%d-%s", time.Now().Unix(), host)
}

func getWithTimeout(url string, timeout time.Duration) (resp *http.Response, err error) {
	start := time.Now()

	for time.Now().Sub(start) < timeout {
		resp, err = http.Get(url)
		if err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	return // Make compiler happy
}

type End2EndTestSuite struct {
	suite.Suite

	logger nuclio.Logger
	cmd    *cmdrunner.CmdRunner

	gopath   string
	oldPath  string
	srcDir   string
	registry string
	testID   string
}

func (suite *End2EndTestSuite) failOnError(err error, fmt string, args ...interface{}) {
	if err != nil {
		suite.FailNow(fmt, args...)
	}
}

func (suite *End2EndTestSuite) gitRoot() string {
	out, err := suite.cmd.Run(nil, "git rev-parse --show-toplevel")
	suite.failOnError(err, "Can't create command runner")
	return strings.TrimSpace(out)
}

func (suite *End2EndTestSuite) SetupSuite() {
	var loggerLevel nucliozap.Level

	if testing.Verbose() {
		loggerLevel = nucliozap.DebugLevel
	} else {
		loggerLevel = nucliozap.InfoLevel
	}
	zap, err := nucliozap.NewNuclioZap("end2end", loggerLevel)
	suite.failOnError(err, "Can't create logger")
	suite.logger = zap
	cmd, err := cmdrunner.NewCmdRunner(suite.logger)
	suite.failOnError(err, "Can't create command runner")
	suite.cmd = cmd

	suite.testID = newTestID()
	suite.logger.InfoWith("Test id", "id", suite.testID)

	gopath, err := ioutil.TempDir("", "e2e-test")
	suite.failOnError(err, "Can't create temp dir for GOPATH")

	suite.registry = fmt.Sprintf("%s:%d", k8sHost, registryPort)

	suite.oldPath = os.Getenv("GOPATH")
	suite.gopath = gopath
	suite.logger.InfoWith("GOPATH", "path", gopath)
	os.Setenv("GOPATH", gopath)

	suite.srcDir = fmt.Sprintf("%s/src/github.com/nuclio/nuclio", gopath)
	suite.logger.InfoWith("Source directory", "path", suite.srcDir)
}

func (suite *End2EndTestSuite) TearDownSuite() {
	os.Setenv("GOPATH", suite.oldPath)
}

func (suite *End2EndTestSuite) getNuclio() {
	suite.logger.InfoWith("Getting nuclio")

	var err error
	root := suite.gitRoot()

	if options.local {
		prjDir := fmt.Sprintf("%s/src/github.com/nuclio/", suite.gopath)
		_, err = suite.cmd.Run(nil, "mkdir -p %s", prjDir)
		suite.failOnError(err, "Can't create %s", prjDir)
		_, err = suite.cmd.Run(nil, "rsync -a %s %s", root, prjDir)
	} else {
		_, err = suite.cmd.Run(nil, "go get github.com/nuclio/nuclio/...")
	}
	suite.failOnError(err, "Can't 'go get' nuclio")
}

func (suite *End2EndTestSuite) cleanK8s() {
	suite.logger.InfoWith("Cleaning k8s from old deployment")

	_, err := suite.cmd.Run(nil, "kubectl delete deploy,rs,ds,svc,po --all")
	suite.failOnError(err, "Can't clear k8s cluster")
}

func (suite *End2EndTestSuite) createController() {
	suite.logger.InfoWith("Creating controller")

	var err error
	opts := &cmdrunner.RunOptions{WorkingDir: &suite.srcDir}
	_, err = suite.cmd.Run(opts, "make")
	suite.failOnError(err, "Can't build controller")

	tag := fmt.Sprintf("%s/%s", suite.registry, imageName)
	_, err = suite.cmd.Run(nil, "docker tag %s %s", imageName, tag)
	suite.failOnError(err, "Can't tag controller image")

	_, err = suite.cmd.Run(nil, "docker push %s", tag)
	suite.failOnError(err, "Can't push controller image")

	ctrlFile := fmt.Sprintf("%s/hack/k8s/resources/controller.yaml", suite.srcDir)
	_, err = suite.cmd.Run(nil, "kubectl create -f %s", ctrlFile)
	suite.failOnError(err, "Can't deploy controller")
}

func (suite *End2EndTestSuite) createHandler() {
	suite.logger.InfoWith("Creating handler")

	var err error
	_, err = suite.cmd.Run(nil, "go get -d github.com/nuclio/nuclio-sdk")
	suite.failOnError(err, "Can't get SDK")
	_, err = suite.cmd.Run(nil, "go get github.com/nuclio/nuclio/cmd/nuclio-build")
	suite.failOnError(err, "Can't get nuclio-build")
	buildDir, err := ioutil.TempDir("", "e2e-test")
	suite.failOnError(err, "Can't create build dir")
	suite.logger.InfoWith("Build directory", "path", buildDir)
	handlerFile, err := os.Create(fmt.Sprintf("%s/handler.go", buildDir))
	suite.failOnError(err, "Can't create handler file")
	suite.logger.InfoWith("Handler file", "path", handlerFile.Name())

	err = handlerTemplate.Execute(handlerFile, suite.testID)
	suite.failOnError(err, "Can't create handler file")
	err = handlerFile.Sync()
	suite.failOnError(err, "Can't sync handler file")

	_, err = suite.cmd.Run(nil, "%s/bin/nuclio-build --name %s --push %s %s", suite.gopath, suite.testID, suite.registry, buildDir)
	suite.failOnError(err, "Can't build")

	params := kubeParame{Tag: suite.testID, Port: HTTPPort}
	cfgFile, err := ioutil.TempFile("", "e2e-config")
	suite.failOnError(err, "Can't create config file")
	suite.logger.InfoWith("config file", "path", cfgFile.Name())
	err = kubeTemplate.Execute(cfgFile, params)
	suite.failOnError(err, "Can't create config file")
	err = cfgFile.Sync()
	suite.failOnError(err, "Can't sync config file")

	// Don't care about error here
	suite.cmd.Run(nil, "kubectl delete -f %s", cfgFile.Name())
	_, err = suite.cmd.Run(nil, "kubectl create --request-timeout 1m -f %s", cfgFile.Name())
	suite.failOnError(err, "Can't create function")
}

func (suite *End2EndTestSuite) callHandler() {
	url := fmt.Sprintf("http://%s:%d", k8sHost, HTTPPort)
	suite.logger.InfoWith("Calling handler", "url", url)

	resp, err := getWithTimeout(url, time.Minute)
	suite.failOnError(err, "Can't call handler")

	defer resp.Body.Close()
	var buf bytes.Buffer
	_, err = io.Copy(&buf, resp.Body)
	if suite.NoError(err, "Can't read response") {
		suite.Assert().Equal(buf.String(), suite.testID, "Bad reply")
	}
}

// TestHTTPFunctionDeploy runs end to end function deploy
func (suite *End2EndTestSuite) TestHTTPFunctionDeploy() {
	suite.getNuclio()
	suite.cleanK8s()
	suite.createController()
	suite.createHandler()
	suite.callHandler()
}

func TestEnd2End(t *testing.T) {
	suite.Run(t, new(End2EndTestSuite))
}
