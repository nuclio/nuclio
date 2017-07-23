package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"text/template"
	"time"
)

const (
	imageName       = "nuclio/controller"
	defaultRegistry = "52.16.125.41:31276"
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

var options struct {
	verbose  bool
	local    bool
	registry string
}

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
	flag.StringVar(&options.registry, "registry", defaultRegistry, "docker registry")
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

	srcDir := fmt.Sprintf("%s/src/github.com/nuclio/nuclio", gopath)
	if err := os.Chdir(srcDir); err != nil {
		die("can't change directory to %s", srcDir)
	}

	if err := runCmd("make", "controller"); err != nil {
		die("can't build controller - %s", err)
	}

	tag := fmt.Sprintf("%s/%s", options.registry, imageName)
	if err := runCmd("docker", "tag", imageName, tag); err != nil {
		die("can't tag image - %s", err)
	}

	if err := runCmd("docker", "push", tag); err != nil {
		die("can't push image - %s", err)
	}

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

	handlerFile, err := ioutil.TempFile("", "e2e-handler")
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

	if err := runCmd("nuclio-build", "-n", testID, "--push", options.registry, handlerFile.Name()); err != nil {
		die("can't build - %s", err)
	}
}
