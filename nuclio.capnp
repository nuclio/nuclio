@0xa6be28a43fb8d71b;

using Java = import "/capnp/java.capnp";
$Java.package("io.nuclio");
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
