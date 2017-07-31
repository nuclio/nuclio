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
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/user"
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
        Body: []byte("{{.TestID}}"),
    }, nil
}
`))

var kubeHandlerTempalte = template.Must(template.New("kubeh").Parse(`
apiVersion: nuclio.io/v1
kind: Function
metadata:
  name: handler
spec:
  replicas: 1
  image: localhost:5000/{{.HandlerTag}}
  httpPort: {{.HTTPPort}}
`))

var kubeRoleTemplate = template.Must(template.New("kubeh").Parse(`
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: {{.TestID}}-service-account
subjects:
  - kind: ServiceAccount
    namespace: {{.TestID}}
    name: default
roleRef:
  kind: ClusterRole
  name: cluster-admin
  apiGroup: rbac.authorization.k8s.io
`))

type kubeServiceResponse struct {
	Items []struct {
		Spec struct {
			Ports []struct {
				NodePort int `json:"nodePort"`
			} `json:"ports"`
		} `json:"spec"`
	} `json:"items"`
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

	var login string
	u, err := user.Current()
	if err != nil {
		login = "unknown-user"
	} else {
		login = u.Username
	}

	return fmt.Sprintf("nuclio-e2e-%d-%s-%s", time.Now().Unix(), host, login)
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

	logger       nuclio.Logger
	cmd          *cmdrunner.CmdRunner
	oldPath      string
	roleFileName string

	GoPath     string
	SourceDir  string
	Registry   string
	TestID     string
	KubeRole   string
	HTTPPort   int
	HandlerTag string
}

func (suite *End2EndTestSuite) failOnError(err error, fmt string, args ...interface{}) {
	if err == nil {
		return
	}
	suite.logger.ErrorWith("Error in test", "error", err)
	suite.FailNow(fmt, args...)
}

func (suite *End2EndTestSuite) gitRoot() string {
	out, err := suite.cmd.Run(nil, "git rev-parse --show-toplevel")
	suite.failOnError(err, "Can't create command runner")
	return strings.TrimSpace(out)
}

func (suite *End2EndTestSuite) nodePort() int {
	out, err := suite.cmd.Run(nil, "kubectl -n %s get svc -o json", suite.TestID)
	suite.failOnError(err, "Can't get service status")

	var resp kubeServiceResponse
	err = json.Unmarshal([]byte(out), &resp)
	suite.failOnError(err, "Can't parse service reply")
	if len(resp.Items) == 0 {
		suite.FailNow("No services found")
	}
	if len(resp.Items[0].Spec.Ports) == 0 {
		suite.FailNow("No ports found")
	}
	return resp.Items[0].Spec.Ports[0].NodePort
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

	suite.TestID = newTestID()
	suite.logger.InfoWith("Test id", "id", suite.TestID)

	gopath, err := ioutil.TempDir("", "e2e-test")
	suite.failOnError(err, "Can't create temp dir for GOPATH")

	suite.Registry = fmt.Sprintf("%s:%d", k8sHost, registryPort)

	suite.oldPath = os.Getenv("GOPATH")
	suite.GoPath = gopath
	suite.logger.InfoWith("GOPATH", "path", gopath)
	os.Setenv("GOPATH", gopath)

	suite.SourceDir = fmt.Sprintf("%s/src/github.com/nuclio/nuclio", gopath)
	suite.logger.InfoWith("Source directory", "path", suite.SourceDir)

	suite.KubeRole = fmt.Sprintf("%s-service-account", suite.TestID)
	suite.HTTPPort = HTTPPort
	suite.HandlerTag = fmt.Sprintf("%s/%s", suite.Registry, imageName)

	suite.createNS()
	suite.createRole()
}

func (suite *End2EndTestSuite) TearDownSuite() {
	os.Setenv("GOPATH", suite.oldPath)
	suite.deleteRole()
	suite.deleteNS()
}

func (suite *End2EndTestSuite) createNS() {
	suite.logger.InfoWith("Creating k8s namespace", "name", suite.TestID)
	_, err := suite.cmd.Run(nil, "kubectl create namespace %s", suite.TestID)
	suite.failOnError(err, "Can't create namespace")
}

func (suite *End2EndTestSuite) deleteNS() {
	suite.logger.InfoWith("Deleting k8s namespace", "name", suite.TestID)
	_, err := suite.cmd.Run(nil, "kubectl delete namespace %s", suite.TestID)
	suite.failOnError(err, "Can't create namespace")
}

func (suite *End2EndTestSuite) createRole() {
	tmpFile, err := ioutil.TempFile("", "e2e-role")
	suite.logger.InfoWith("Creating role", "name", suite.KubeRole, "path", tmpFile.Name())
	suite.roleFileName = tmpFile.Name()
	suite.failOnError(err, "Can't create role file")
	err = kubeRoleTemplate.Execute(tmpFile, suite)
	suite.failOnError(err, "Can't execute role template")
	err = tmpFile.Close()
	suite.failOnError(err, "Can't execute role template")
}

func (suite *End2EndTestSuite) deleteRole() {
	_, err := suite.cmd.Run(nil, "kubectl delete -f %s", suite.roleFileName)
	suite.failOnError(err, "can't delete role")
}

func (suite *End2EndTestSuite) getNuclio() {
	suite.logger.InfoWith("Getting nuclio")

	var err error
	root := suite.gitRoot()

	if options.local {
		prjDir := fmt.Sprintf("%s/src/github.com/nuclio/", suite.GoPath)
		_, err = suite.cmd.Run(nil, "mkdir -p %s", prjDir)
		suite.failOnError(err, "Can't create %s", prjDir)
		_, err = suite.cmd.Run(nil, "rsync -a %s %s", root, prjDir)
	} else {
		_, err = suite.cmd.Run(nil, "go get github.com/nuclio/nuclio/...")
	}
	suite.failOnError(err, "Can't 'go get' nuclio")
}

func (suite *End2EndTestSuite) createController() {
	suite.logger.InfoWith("Creating controller")

	var err error
	opts := &cmdrunner.RunOptions{WorkingDir: &suite.SourceDir}
	_, err = suite.cmd.Run(opts, "make")
	suite.failOnError(err, "Can't build controller")

	_, err = suite.cmd.Run(nil, "docker tag %s %s", imageName, tag)
	suite.failOnError(err, "Can't tag controller image")

	_, err = suite.cmd.Run(nil, "docker push %s", tag)
	suite.failOnError(err, "Can't push controller image")

	ctrlFile := fmt.Sprintf("%s/hack/k8s/resources/controller.yaml", suite.SourceDir)
	_, err = suite.cmd.Run(nil, "kubectl --namespace %s create -f %s", suite.TestID, ctrlFile)
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

	err = handlerTemplate.Execute(handlerFile, suite)
	suite.failOnError(err, "Can't create handler file")
	err = handlerFile.Sync()
	suite.failOnError(err, "Can't sync handler file")

	_, err = suite.cmd.Run(nil, "%s/bin/nuclio-build --name %s --push %s %s", suite.GoPath, suite.TestID, suite.Registry, buildDir)
	suite.failOnError(err, "Can't build")

	cfgFile, err := ioutil.TempFile("", "e2e-config")
	suite.failOnError(err, "Can't create config file")
	suite.logger.InfoWith("config file", "path", cfgFile.Name())
	err = kubeHandlerTempalte.Execute(cfgFile, suite)
	suite.failOnError(err, "Can't create config file")
	err = cfgFile.Sync()
	suite.failOnError(err, "Can't sync config file")

	// Don't care about error here
	_, err = suite.cmd.Run(nil, "kubectl --namespace %s create --request-timeout 1m -f %s", suite.TestID, cfgFile.Name())
	suite.failOnError(err, "Can't create function")
}

func (suite *End2EndTestSuite) callHandler() {
	port := suite.nodePort()
	url := fmt.Sprintf("http://%s:%d", k8sHost, port)
	suite.logger.InfoWith("Calling handler", "url", url)

	resp, err := getWithTimeout(url, time.Minute)
	suite.failOnError(err, "Can't call handler")

	defer resp.Body.Close()
	var buf bytes.Buffer
	_, err = io.Copy(&buf, resp.Body)
	if suite.NoError(err, "Can't read response") {
		suite.Assert().Equal(buf.String(), suite.TestID, "Bad reply")
	}
}

// TestHTTPFunctionDeploy runs end to end function deploy
func (suite *End2EndTestSuite) TestHTTPFunctionDeploy() {
	suite.getNuclio()
	suite.createController()
	suite.createHandler()
	suite.callHandler()
}

func TestEnd2End(t *testing.T) {
	suite.Run(t, new(End2EndTestSuite))
}
