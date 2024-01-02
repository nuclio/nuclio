package utils

import (
	"github.com/stretchr/testify/suite"
	"testing"
)

type HelperSuite struct {
	suite.Suite
}

func (helperSuite *HelperSuite) TestGetEnvironmentHost() {
	hostEnv := GetEnvironmentHost()

	helperSuite.Equal("localhost", hostEnv)
}

func TestSuite(t *testing.T) {
	suite.Run(t, new(HelperSuite))
}
