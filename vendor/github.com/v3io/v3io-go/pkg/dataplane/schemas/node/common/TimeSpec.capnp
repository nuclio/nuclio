@0xbcbc7bd29390d6e8;
using Go = import "/go.capnp";
$Go.package("node_common_capnp");
$Go.import("github.com/v3io/v3io-go/internal/schemas/node/common");

# Imports & Namespace settings
using Java = import "/java/java.capnp";
$Java.package("io.iguaz.v3io.daemon.client.api.capnp");
$Java.outerClassname("V3ioTimeSpec");

struct TimeSpec {
    tvSec  @0 : Int64;
    tvNsec @1 : Int64;
}

