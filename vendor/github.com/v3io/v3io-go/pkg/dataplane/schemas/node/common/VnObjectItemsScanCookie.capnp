@0xb56ec2d13b48b7cb;
using Go = import "/go.capnp";
$Go.package("node_common_capnp");
$Go.import("github.com/v3io/v3io-go/internal/schemas/node/common");

# Imports & Namespace settings
using Java = import "/java/java.capnp";
$Java.package("io.iguaz.v3io.daemon.client.api.capnp");
$Java.outerClassname("VnObjectItemsScanCookieOuter");

struct VnObjectItemsScanCookie {
    sliceId            @0 :UInt16;
    inodeNumber        @1 :UInt32;
    clientSliceListPos @2 :UInt64;
    clientSliceListEndPos @3 :UInt64;
}
