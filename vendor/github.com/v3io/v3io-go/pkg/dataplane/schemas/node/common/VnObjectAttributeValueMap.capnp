@0x90687959836864ab;
using Go = import "/go.capnp";
$Go.package("node_common_capnp");
$Go.import("github.com/v3io/v3io-go/internal/schemas/node/common");

using Java = import "/java/java.capnp";
using import "/node/common/ExtAttrValue.capnp".ExtAttrValue;
$Java.package("io.iguaz.v3io.daemon.client.api.capnp");
$Java.outerClassname("VnObjectAttributeValueMapOuter");

struct VnObjectAttributeValuePtr {
	value @0 : ExtAttrValue;
}

struct VnObjectAttributeValueMap {
	values @0 : List(VnObjectAttributeValuePtr);
}