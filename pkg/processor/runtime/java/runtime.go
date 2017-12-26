/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package java

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/nuclio/nuclio-sdk"
	"zombiezen.com/go/capnproto2"
)

const (
	mmapSize          = 4 * (1 << 20) // 4MB
	connectionTimeout = 10 * time.Second
	eventTimeout      = 5 * time.Minute
)

type javaResult struct {
	resp *Response
	err  error
}

type java struct {
	runtime.AbstractRuntime
	configuration *runtime.Configuration
	tmpDir        string
	inFifo        *os.File
	outFifo       *os.File
	mmapPath      string
	sharedMem     *BytesIO
	encoder       *capnp.Encoder
	rwBuf         []byte
	resultChan    chan *javaResponse
}

// NewRuntime returns a new Java runtime
func NewRuntime(parentLogger nuclio.Logger, configuration *runtime.Configuration) (runtime.Runtime, error) {
	logger := parentLogger.GetChild("java")

	var err error

	tmpDir, err := ioutil.TempDir("", "nuclio-java")
	if err != nil {
		return nil, errors.Wrap(err, "Can't create temp directory")
	}

	abstractRuntime, err := runtime.NewAbstractRuntime(logger, configuration)
	if err != nil {
		return nil, errors.Wrap(err, "Can't create AbstractRuntime")
	}

	newJavaRuntime := &java{
		AbstractRuntime: *abstractRuntime,
		configuration:   configuration,
		tmpDir:          tmpDir,
		rwBuf:           make([]byte, 1),
	}

	if err := newJavaRuntime.createFifos(); err != nil {
		return nil, err
	}

	if err := newJavaRuntime.createMmap(); err != nil {
		return nil, err
	}
	newJavaRuntime.encoder = capnp.NewEncoder(newJavaRuntime.sharedMem)
}

func (j *java) createFifos() error {
	outPath := path.Join(j.tmpDir, "runtime-out")
	if common.FileExists(outPath) {
		if err := os.Remove(outPath); err != nil {
			return errors.Wrapf(err, "Can't delete %q", outPath)
		}
	}

	if err := syscall.Mkfifo(outPath, 0600); err != nil {
		return errors.Wrapf(err, "Can't create fifo at %q", outPath)
	}

	out, err := os.OpenFile(outPath, os.O_RDWR, 0600)
	if err != nil {
		return errors.Wrapf(err, "Can't open %q", outPath)
	}

	j.outFifo = out
	j.Logger.InfoWith("Output fifo created", "path", outPath)

	inPath := path.Join(j.tmpDir, "runtime-in")
	if fileExists(inPath) {
		if err := os.Remove(inPath); err != nil {
			return errors.Wrapf(err, "Can't delete %q", inPath)
		}
	}

	if err := syscall.Mkfifo(inPath, 0600); err != nil {
		return errors.Wrapf(err, "Can't create fifo at %q", inPath)
	}

	in, err := os.OpenFile(inPath, os.O_RDWR, 0600)
	if err != nil {
		return errors.Wrapf(err, "Can't open %q", inPath)
	}

	j.inFifo = in
	j.Logger.InfoWith("Input fifo created", "path", inPath)

	if err := j.runWrapper(); err != nil {
		return nil, err
	}
	return nil
}

func (j *java) createMmap() error {
	j.mmapPath = path.Join(j.tmpDir, "mmap")
	file, err := os.OpenFile(j.mmapPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return errors.Wrapf(err, "Can't create file at %q", j.mmapPath)
	}

	j.Logger.InfoWith("mmap file created", "path", j.mmapPath)

	fd := int(file.Fd())
	err = syscall.Ftruncate(fd, mmapSize)
	if err != nil {
		return errors.Wrapf(err, "Can't resize %q", j.mmapPath)
	}

	flags := syscall.PROT_WRITE | syscall.PROT_READ
	buf, err := syscall.Mmap(int(file.Fd()), 0, mmapSize, flags, syscall.MAP_SHARED)
	if err != nil {
		return errors.Wrapf(err, "Can't create mmap on %q", j.mmapPath)
	}

	j.sharedMem = NewBytesIO(buf)
	return nil
}

func (j *java) encodeEvent(event nuclio.Event) (*capnp.Message, error) {
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	cEvent, err := NewRootEvent(seg)
	if err != nil {
		return nil, err
	}

	cEvent.SetVersion(int64(event.GetVersion()))
	cEvent.SetId(event.GetID().String())

	src, err := cEvent.NewSource()
	if err != nil {
		return nil, err
	}
	sinfo := event.GetSource()
	src.SetClassName(sinfo.GetClass())
	src.SetKindName(sinfo.GetKind())

	cEvent.SetContentType(event.GetContentType())
	cEvent.SetBody(event.GetBody())
	cEvent.SetSize(int64(event.GetSize()))

	// Encode fields map as list of entries
	// TODO: Unite with fields encoding below
	hdrs, err := cEvent.NewHeaders(int32(len(event.GetHeaders())))
	i := 0
	for key, val := range event.GetHeaders() {
		hdrs.At(i).SetKey(key)
		switch val.(type) {
		case string:
			hdrs.At(i).Value().SetSVal(val.(string))
		case int:
			hdrs.At(i).Value().SetIVal(int64(val.(int)))
		case []byte:
			hdrs.At(i).Value().SetDVal(val.([]byte))
		default:
			return nil, fmt.Errorf("uknown header type for %s - %T", key, val)
		}
	}

	cEvent.SetTimestamp(event.GetTimestamp().UnixNano() / int64(time.Millisecond))
	cEvent.SetPath(event.GetPath())
	cEvent.SetUrl(event.GetURL())
	cEvent.SetMethod(event.GetMethod())

	// Encode fields map as list of entries
	fields, err := cEvent.NewFields(int32(len(event.GetFields())))
	if err != nil {
		return nil, err
	}
	i = 0
	for key, val := range event.GetFields() {
		hdrs.At(i).SetKey(key)
		switch val.(type) {
		case string:
			fields.At(i).Value().SetSVal(val.(string))
		case int:
			fields.At(i).Value().SetIVal(int64(val.(int)))
		case []byte:
			hdrs.At(i).Value().SetDVal(val.([]byte))
		default:
			return nil, fmt.Errorf("uknown header type for %s - %T", key, val)
		}
	}

	return msg, nil
}

func (j *java) readResponse() (Response, error) {
	msg, err := capnp.NewDecoder(j.sharedMem).Decode()
	if err != nil {
		return nil, err
	}

	resp, err := ReadRootResponse(msg)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// TODO: Wait for connection (see connectionTimeout)
func (j *java) runWrapper() error {
	jarPath = j.wrapperJarPath()
	// TODO: user jar and class from it
	cmd := exec.Command("java", "-jar", jarPath, j.outFifo.Name(), j.inFifo.Name())
	j.Logger.InfoWith("Running wrapper jar", "command", strings.Join(cmd.Args, ""))

	return cmd.Start()
}

func (j *java) wrapperJarPath() {
	wrapperPath := os.Getenv("NUCLIO_WRAPPER_JAR")
	if wrapperPath != "" {
		return wrapperPath
	}

	return "/opt/nuclio/nuclio-wrapper.jar"
}

func (j *java) handleEvent(event nuclio.Event) {
	result := &javaResult{}

	j.bw.Reset()
	if err := j.enc.Encode(msg); err != nil {
		result.err = errors.Wrap(err, "Can't encode event")
		j.resultChan <- result
		return
	}

	j.rwBuf[0] = 'e'
	if _, err := j.outFifo.Write(rwBuf); err != nil {
		result.err = errors.Wrap(err, "Can't signal to java")
		j.resultChan <- result
		return
	}

	if _, err = in.Read(rwBuf); err != nil {
		result.err = errors.Wrap(err, "Can't get signal from java")
		j.resultChan <- result
	}

	bw.Seek(0)
	result.resp, result.err = readResponse(bw)
	j.resultChan <- result
}

func (j *java) ProcessEvent(event nuclio.Event, functionLogger nuclio.Logger) (interface{}, error) {
	j.Logger.DebugWith("Processing event",
		"name", j.configuration.Meta.Name,
		"version", j.configuration.Spec.Version,
		"eventID", event.GetID())

	go j.handleEvent(event)
	select {
	case result := <-j.resultChan:
		j.Logger.DebugWith("Python executed",
			"status", result.resp.Status(),
			"eventID", event.ID())

		// TODO: Convert to response

		return nuclio.Response{}, nil
	case <-time.After(eventTimeout):
		return nil, fmt.Errorf("handler timeout after %s", eventTimeout)
	}
}
