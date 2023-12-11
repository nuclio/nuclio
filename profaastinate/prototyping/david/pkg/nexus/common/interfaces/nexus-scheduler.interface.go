package interfaces

import (
	"github.com/konsumgandalf/mpga-protoype-david/pkg/nexus/common/structs"
)

type INexusScheduler interface {
	Push(item structs.BaseNexusItem)
	Remove(ID string)
	Pop(ID string) structs.BaseNexusItem
}
