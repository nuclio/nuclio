@0xdf50359faf84cbef;
using Go = import "/go.capnp";
$Go.package("node_common_capnp");
$Go.import("github.com/v3io/v3io-go/internal/schemas/node/common");

using Java = import "/java/java.capnp";
$Java.package("io.iguaz.v3io.daemon.client.api.capnp");
$Java.outerClassname("StringWrapperOuter");

struct StringWrapper {
    str @0 : Text;
}

struct StringWrapperList{
    arr @0 : List(StringWrapper);
}
