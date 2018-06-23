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

package httpsrv

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/nuclio/nuclio/pkg/errors"
)

// ServedObject represents an object that will be returned at a given pattern
type ServedObject struct {
	Contents string
	Pattern  string
}

// ServedObject represents a file that will be returned at a given pattern
type ServedFile struct {
	LocalPath string
	Pattern   string
}

// Server serves objects and files
type Server struct {
	http.Server
}

// NewServer creates an object/file server
func NewServer(addr string,
	servedFiles []ServedFile,
	servedObjects []ServedObject) (*Server, error) {
	var err error

	newServer := Server{}

	// if user didn't pass an address, generate one
	if addr == "" {
		addr, err = generateAddress()

		if err != nil {
			return nil, errors.Wrap(err, "Failed to find free port")
		}
	}

	newServer.Addr = addr

	// create a new servemux
	newServeMux := http.NewServeMux()
	newServer.Handler = newServeMux

	// register files
	for _, servedFile := range servedFiles {
		servedFileCopy := servedFile

		newServeMux.HandleFunc("/"+servedFileCopy.Pattern, func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, servedFileCopy.LocalPath)
		})

	}

	// register objects
	for _, servedObject := range servedObjects {
		servedObjectCopy := servedObject

		newServeMux.HandleFunc("/"+servedObjectCopy.Pattern, func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(servedObjectCopy.Contents))
		})
	}

	go newServer.ListenAndServe() // nolint: errcheck

	return &newServer, nil
}

// Stop stops serving
func (s *Server) Stop() error {
	return s.Shutdown(context.TODO())
}

func generateAddress() (string, error) {
	freePort, err := findFreePort()
	if err != nil {
		return "", errors.Wrap(err, "Failed to find free port")
	}

	return fmt.Sprintf("127.0.0.1:%s", freePort.Port), nil
}

func findFreePort() (*net.TCPAddr, error) {
	return net.ResolveTCPAddr("tcp", "localhost:0")
}
