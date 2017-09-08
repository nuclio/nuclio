package web_interface

import (
	"github.com/nuclio/nuclio/pkg/util/registry"
)

var ResourceRegistrySingleton = registry.NewRegistry("web_interface_resource")
