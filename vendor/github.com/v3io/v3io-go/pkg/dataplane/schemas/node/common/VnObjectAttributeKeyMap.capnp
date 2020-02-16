@0x986bf57944c8b89f;
using Go = import "/go.capnp";
$Go.package("node_common_capnp");
$Go.import("github.com/v3io/v3io-go/internal/schemas/node/common");

using Java = import "/java/java.capnp";
using import "/node/common/StringWrapper.capnp".StringWrapperList;
$Java.package("io.iguaz.v3io.daemon.client.api.capnp");
$Java.outerClassname("VnObjectAttributeKeyMapOuter");

struct VnObjectAttributeKeyMap {
	names @0 : StringWrapperList;
}
