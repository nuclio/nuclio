package mock

import "github.com/stretchr/testify/mock"

//
// KubePlatform mock
//

// Platform defines the interface that any underlying function platform must provide for nuclio
// to run over it
type Platform struct {
	mock.Mock
}

//
// Function
//

func (mp *Platform) GetContainerBuilderKind() string {
	return "docker"
}
