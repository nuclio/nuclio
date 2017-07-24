/* End to end test

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
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"text/template"
	"time"
)

const (
	imageName           = "nuclio/controller"
	defaultHost         = "52.16.125.41"
	defaultRegistryPort = 31276
	defaultHTTPPort     = 31010
)

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

var options struct {
	verbose      bool
	local        bool
	k8sHost      string
	registryPort int
	port         int
}

// io.Writer interface to logs
type logWriter struct {
	prefix string
	data   []byte
}

func (lw *logWriter) Write(p []byte) (n int, err error) {
	for _, b := range p {
		if b == '\n' {
			lw.Flush()
			continue
		}
		lw.data = append(lw.data, b)
	}

	return len(p), nil
}

func (lw *logWriter) Flush() {
	if len(lw.data) > 0 {
		log.Printf("%s %s", lw.prefix, string(lw.data))
		lw.data = []byte{}
	}
}

func runCmd(cmdLine ...string) error {
	log.Printf(strings.Join(cmdLine, " "))

	cmd := exec.Command(cmdLine[0], cmdLine[1:]...)
	if options.verbose {
		lwo := &logWriter{prefix: "<stdout>"}
		defer lwo.Flush()
		cmd.Stdout = lwo

		lwe := &logWriter{prefix: "<stderr>"}
		defer lwe.Flush()
		cmd.Stderr = lwe
	} else {
		cmd.Stdout = ioutil.Discard
		cmd.Stderr = ioutil.Discard
	}

	return cmd.Run()
}

func gitRoot() (string, error) {
	var buf bytes.Buffer
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Stdout = &buf
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

func getNuclio(gopath string) error {
	var cmdLine []string
	if options.local {
		root, err := gitRoot()
		if err != nil {
			return err
		}
		dest := fmt.Sprintf("%s/src/github.com/nuclio", gopath)
		if err := runCmd("mkdir", "-p", dest); err != nil {
			return err
		}
		cmdLine = []string{"rsync", "-a", root, dest}
	} else {
		cmdLine = []string{"go", "get"}
		if options.verbose {
			cmdLine = append(cmdLine, "-v")
		}
		cmdLine = append(cmdLine, "github.com/nuclio/nuclio/...")
	}

	if err := runCmd(cmdLine...); err != nil {
		log.Printf("error getting nuclio")
		return err
	}
	return nil
}

// newTestID return new unique test ID
func newTestID() (string, error) {
	host, err := os.Hostname()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("nuclio-e2e-%d-%s", time.Now().Unix(), host), nil
}

func die(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "error: %s\n", msg)
	os.Exit(1)
}

func main() {
	flag.BoolVar(&options.local, "local", false, "get local copy of nuclio")
	flag.BoolVar(&options.verbose, "verbose", false, "be verbose")
	flag.StringVar(&options.k8sHost, "k8sHost", defaultHost, "k8s host")
	flag.IntVar(&options.registryPort, "registryPort", defaultRegistryPort, "docker registry port")
	flag.IntVar(&options.port, "port", defaultHTTPPort, "handler HTTP port")
	flag.Parse()

	if !options.verbose {
		log.SetOutput(ioutil.Discard)
	}

	gopath, err := ioutil.TempDir("", "e2e-test")
	if err != nil {
		die("can't create temp dir - %s", err)
	}
	log.Printf("GOPATH=%s", gopath)
	os.Setenv("GOPATH", gopath)
	if err := getNuclio(gopath); err != nil {
		die("can't get nuclio - %s", err)
	}

	log.Printf("cleaning k8s from old deployment")
	if err := runCmd("kubectl", "delete", "deploy,rs,ds,svc,po", "--all"); err != nil {
		die("can't clear k8s cluster")
	}

	log.Printf("building controller")
	srcDir := fmt.Sprintf("%s/src/github.com/nuclio/nuclio", gopath)
	if err := os.Chdir(srcDir); err != nil {
		die("can't change directory to %s", srcDir)
	}

	if err := runCmd("make"); err != nil {
		die("can't build controller - %s", err)
	}

	registry := fmt.Sprintf("%s:%d", options.k8sHost, options.registryPort)
	tag := fmt.Sprintf("%s/%s", registry, imageName)
	if err := runCmd("docker", "tag", imageName, tag); err != nil {
		die("can't tag controller image - %s", err)
	}

	if err := runCmd("docker", "push", tag); err != nil {
		die("can't push controller image - %s", err)
	}

	ctrlFile := fmt.Sprintf("%s/hack/k8s/resources/controller.yaml", srcDir)
	if err := runCmd("kubectl", "create", "-f", ctrlFile); err != nil {
		die("can't deploy controller - %s", err)
	}

	log.Printf("building handler")
	if err := runCmd("go", "get", "-d", "github.com/nuclio/nuclio-sdk"); err != nil {
		die("can't get SDK - %s", err)
	}

	if err := runCmd("go", "get", "github.com/nuclio/nuclio/cmd/nuclio-build"); err != nil {
		die("can't get nuclio-build - %s", err)
	}

	testID, err := newTestID()
	if err != nil {
		die("can't generate reply - %s", err)
	}
	log.Printf("test ID: %s", testID)

	buildDir, err := ioutil.TempDir("", "e2e-test")
	if err != nil {
		die("can't create build dir - %s", err)
	}
	log.Printf("build dir: %s", buildDir)

	handlerFile, err := os.Create(fmt.Sprintf("%s/handler.go", buildDir))
	if err != nil {
		die("can't create handler file - %s", err)
	}
	log.Printf("handler file: %s", handlerFile.Name())

	if err := handlerTemplate.Execute(handlerFile, testID); err != nil {
		die("can't create handler file - %s", err)
	}
	if err := handlerFile.Sync(); err != nil {
		die("can't sync handler file - %s", err)
	}

	nbExe := fmt.Sprintf("%s/bin/nuclio-build", gopath)
	if err := runCmd(nbExe, "-n", testID, "--push", registry, buildDir); err != nil {
		die("can't build - %s", err)
	}

	params := kubeParame{Tag: testID, Port: options.port}
	cfgFile, err := ioutil.TempFile("", "e2e-config")
	if err != nil {
		die("can't create config file - %s", err)
	}
	log.Printf("config file: %s", cfgFile.Name())
	if err := kubeTemplate.Execute(cfgFile, params); err != nil {
		die("can't create config file - %s", err)
	}
	if err := cfgFile.Sync(); err != nil {
		die("can't sync config file - %s", err)
	}

	// Don't care about error here
	runCmd("kubectl", "delete", "-f", cfgFile.Name())
	if err := runCmd("kubectl", "create", "--request-timeout", "1m", "-f", cfgFile.Name()); err != nil {
		die("can't create function - %s", err)
	}

	time.Sleep(1 * time.Second) // Let controller get up
	url := fmt.Sprintf("http://%s:%d", options.k8sHost, options.port)
	log.Printf("calling handler at %q", url)
	resp, err := http.Get(url)
	if err != nil {
		die("can't call handler - %s", err)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, resp.Body); err != nil {
		die("can't read response - %s", err)
	}

	if buf.String() != testID {
		die("bad reply: got %q, expected %q", buf.String(), testID)
	}
	log.Printf("OK")
}
