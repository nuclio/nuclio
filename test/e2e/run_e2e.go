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
)

var options struct {
	verbose bool
	local   bool
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

func runCmd(cmdLine []string) error {
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

func getRoot() (string, error) {
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
		root, err := getRoot()
		if err != nil {
			return err
		}
		dest := fmt.Sprintf("%s/src/github.com/nuclio", gopath)
		if err := runCmd([]string{"mkdir", "-p", dest}); err != nil {
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

	if err := runCmd(cmdLine); err != nil {
		log.Printf("error getting nuclio")
		return err
	}
	return nil
}

func die(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "error: %s\n", msg)
	os.Exit(1)
}

func main() {
	flag.BoolVar(&options.local, "local", false, "get local copy of nuclio")
	flag.BoolVar(&options.verbose, "verbose", false, "be verbose")
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

	if err := runCmd([]string{"make", "controller"}); err != nil {
		die("can't build controller - %s", err)
	}
}
