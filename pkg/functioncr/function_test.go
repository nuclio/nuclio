package functioncr

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type FunctionTestSuite struct {
	suite.Suite
	function Function
}

func (suite *FunctionTestSuite) TestOnlyName() {
	suite.function.Name = "just_name"

	name, version, err := suite.function.GetNameAndVersion()
	suite.NoError(err)
	suite.Equal("just_name", name)
	suite.Nil(version)
}

func (suite *FunctionTestSuite) TestNameAndVersion() {
	suite.function.Name = "111ValidName123-30"

	name, version, err := suite.function.GetNameAndVersion()
	suite.NoError(err)
	suite.Equal("111ValidName123", name)
	suite.Equal(30, *version)
}

func (suite *FunctionTestSuite) TestInvalidName() {
	suite.function.Name = ""
	_, _, err := suite.function.GetNameAndVersion()
	suite.Error(err)

	suite.function.Name = "@invalidchars"
	_, _, err = suite.function.GetNameAndVersion()
	suite.Error(err)

	suite.function.Name = "valid-invalidversion"
	_, _, err = suite.function.GetNameAndVersion()
	suite.Error(err)
}

func (suite *FunctionTestSuite) TestGetNamespacedName() {
	suite.function.Name = "namepart"
	suite.function.Namespace = "namespacepart"

	namepacedName := suite.function.GetNamespacedName()
	suite.Equal("namespacepart.namepart", namepacedName)
}

func TestFunctionTestSuite(t *testing.T) {
	suite.Run(t, new(FunctionTestSuite))
}
