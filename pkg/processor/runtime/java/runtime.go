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
	"syscall"
	"time"

	"github.com/nuclio/nuclio-sdk"
	"zombiezen.com/go/capnproto2"
)

const (
	mmapSize = 4 * (1 << 20) // 4MB
)

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func createFifos() (io.Writer, io.Reader, error) {
	tmpDir, err := ioutil.TempDir("", "fifo")
	if err != nil {
		return nil, nil, err
	}

	outPath := fmt.Sprintf("%s/srv-out", tmpDir)
	if fileExists(outPath) {
		if err := os.Remove(outPath); err != nil {
			return nil, nil, err
		}
	}

	if err := syscall.Mkfifo(outPath, 0600); err != nil {
		return nil, nil, err
	}

	out, err := os.OpenFile(outPath, os.O_RDWR, 0600)
	if err != nil {
		return nil, nil, err
	}

	inPath := fmt.Sprintf("%s/srv-in", tmpDir)
	if fileExists(inPath) {
		if err := os.Remove(inPath); err != nil {
			return nil, nil, err
		}
	}

	if err := syscall.Mkfifo(inPath, 0600); err != nil {
		return nil, nil, err
	}

	in, err := os.OpenFile(inPath, os.O_RDWR, 0600)
	if err != nil {
		return nil, nil, err
	}

	return out, in, nil
}

func encode(event nuclio.Event) (*capnp.Message, error) {
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

	fields, err := cEvent.NewFields(int32(len(event.GetFields())))
	if err != nil {
		return nil, err
	}

	i = 0
	for key, val := range event.GetHeaders() {
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

func readResponse(r io.Reader) Response {
	msg, err := capnp.NewDecoder(r).Decode()
	if err != nil {
		panic(err)
	}

	resp, err := ReadRootResponse(msg)
	if err != nil {
		panic(err)
	}

	return resp
}

func createMmap(filename string) ([]byte, error) {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}

	fd := int(file.Fd())
	err = syscall.Ftruncate(fd, mmapSize)
	if err != nil {
		return nil, err
	}

	flags := syscall.PROT_WRITE | syscall.PROT_READ
	buf, err := syscall.Mmap(int(file.Fd()), 0, mmapSize, flags, syscall.MAP_SHARED)
	if err != nil {
		return nil, err
	}

	return buf, nil
}

func main() {
	out, in, err := createFifos()
	if err != nil {
		panic(err)
	}

	buf, err := createMmap("/tmp/buff")
	if err != nil {
		panic(err)
	}
	bw := NewBytesIO(buf)
	enc := capnp.NewEncoder(bw)

	evt := NewNuclioDummyEvent()
	msg, err := encode(evt)
	if err != nil {
		panic(err)
	}

	rwBuf := make([]byte, 1)

	start := time.Now()
	nreqs := 10000

	for i := 0; i < nreqs; i++ {
		bw.Reset()
		if err := enc.Encode(msg); err != nil {
			panic(err)
		}

		rwBuf[0] = 'e'
		_, err = out.Write(rwBuf)
		if err != nil {
			panic(err)
		}

		_, err = in.Read(rwBuf)
		if err != nil {
			panic(err)
		}

		//fmt.Println("RESPONSE")
		bw.Seek(0)
		readResponse(bw)
		/*
			resp := readResponse(bw)
			body, err := resp.Body()
			if err != nil {
				panic(err)
			}
			fmt.Println("BODY: ", string(body))
		*/
	}

	since := time.Since(start)
	duration := float64(since) / float64(time.Second)
	rps := float64(nreqs) / duration
	fmt.Printf("%.2fRPS (duration=%s, nreqs=%d)\n", rps, since, nreqs)
}
