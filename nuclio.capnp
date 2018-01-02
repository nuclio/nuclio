# Copyright 2017 The Nuclio Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#	 http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

@0xa6be28a43fb8d71b;

using Java = import "/capnp/java.capnp";
$Java.package("io.nuclio.wrapper");
$Java.outerClassname("NuclioIPC");

using Go = import "/go.capnp";
# TODO: Place in shared located?
$Go.package("java");
# TODO: What does this do?
$Go.import("github.com/nuclio/nuclio-ipc");

struct SourceProvider {
    className @0 :Text;
    kindName @1 :Text;
}

struct Entry {
    key @0 :Text;
    value :union {
		sVal @1 :Text;
		iVal @2 :Int64;
		dVal @3 :Data;
		fVal @4 :Float64;
    }
}


struct Event {
    version @0 :Int64;
    id @1 :Text;
    source @2 :SourceProvider;
    contentType @3 :Text;
    body @4 :Data;
    size @5 :Int64;
    headers @6 :List(Entry);
    fields @7 :List(Entry);
    timestamp @8 :Int64; # milliseconds since epoch
    path @9 :Text;
    url @10 :Text;
    method @11 :Text;
}

struct Response {
    body @0 :Data;
    contentType @1 :Text;
    status @2 :Int64;
    headers @3 :List(Entry);
}

struct LogRecord {
    level @0 :Level;
    message @1 :Text;
    timestamp @2 :Int64; # milliseconds since epoch
    with @3 :List(Entry);

    enum Level {
	error @0;
	warning @1;
	info @2;
	debug @3;
    }
}
