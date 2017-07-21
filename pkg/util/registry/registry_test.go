package registry

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type RegistryTestSuite struct {
	suite.Suite
}

func (suite *RegistryTestSuite) TestRegistration() {
	r := NewRegistry("myclass")

	kind1Value := 1
	kind2Value := "kind2 value"

	// register two classes
	r.Register("kind1", kind1Value)
	r.Register("kind2", kind2Value)

	// re-registering should panic
	suite.Panics(func() {r.Register("kind1", kind1Value)})

	// get kinds
	kinds := r.GetKinds()
	suite.Len(kinds, 2)
	suite.Contains(kinds, "kind1")
	suite.Contains(kinds, "kind2")

	// get known
	v, err := r.Get("kind1")
	suite.NoError(err)
	suite.Equal(kind1Value, v.(int))

	v, err = r.Get("kind2")
	suite.NoError(err)
	suite.Equal(kind2Value, v.(string))

	// get unknown
	v, err = r.Get("unknown")
	suite.Error(err)
	suite.Nil(v)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestRegistryTestSuite(t *testing.T) {
	suite.Run(t, new(RegistryTestSuite))
}
