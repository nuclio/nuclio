// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package apipb

// Run "go generate" in this directory to update. You need to have:
//
// - a protoc binary (see https://github.com/golang/protobuf#installation)
// - go get grpc-codegen.go4.org/protoc-gen-go4grpc

//go:generate protoc --proto_path=$GOPATH/src:. --go4grpc_out=plugins=grpc:. api.proto
