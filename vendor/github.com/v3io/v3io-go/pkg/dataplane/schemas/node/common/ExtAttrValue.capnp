@0x8a6e4e6e3e2db81e;
using Go = import "/go.capnp";
$Go.package("node_common_capnp");
$Go.import("github.com/v3io/v3io-go/internal/schemas/node/common");

using Java = import "/java/java.capnp";
$Java.package("io.iguaz.v3io.daemon.client.api.capnp");
$Java.outerClassname("ExtAttrValueOuter");

using import "/node/common/TimeSpec.capnp".TimeSpec;

struct ExtAttrValue{
    union {
       qword                @0 : Int64;
       uqword               @1 : UInt64;
       blob                 @2 : Data;
       notExists            @3 : Void;
       str                  @4 : Text;
       qwordIncrement       @5 : Int64;
       time                 @6 : TimeSpec;
       dfloat               @7 : Float64;
       floatIncrement       @8 : Float64;
       boolean              @9 : Bool;
    }
}
