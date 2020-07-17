package cors

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	suite.Suite
	cors *CORS
}

func (suite *TestSuite) SetupSuite() {
	suite.cors = NewCORS()
	suite.cors.Enabled = true
}

func (suite *TestSuite) TestEncodeAllowCredentialsHeader() {

	// false by default
	suite.Require().Equal(suite.cors.EncodeAllowCredentialsHeader(), "false")

	// empty lazy-load encoded string
	suite.cors.allowCredentialsStr = ""
	suite.cors.AllowCredentials = true
	suite.Require().Equal(suite.cors.EncodeAllowCredentialsHeader(), "true")
}

func (suite *TestSuite) TestOriginAllowed() {
	dummyHostA := "a.host"
	dummyHostB := "b.host"

	// regardless to allow origin, empty host is against CORS RFC and should not be treated
	suite.Require().False(suite.cors.OriginAllowed(""))

	// allow all by default
	originHosts := []string{
		dummyHostA,
		dummyHostB,
	}
	for _, originHost := range originHosts {
		suite.Require().True(suite.cors.OriginAllowed(originHost))
	}

	// allow for a specific host only
	err := suite.cors.SetAllowOriginURL(dummyHostA)
	suite.Require().NoError(err)
	suite.Require().False(suite.cors.OriginAllowed(dummyHostB))
	suite.Require().True(suite.cors.OriginAllowed(dummyHostA))

	testBaseHost := "mynaipi.com:80"
	err = suite.cors.SetAllowOriginURL(dummyHostA)
	suite.Require().NoError(err)
	suite.Require().True(suite.cors.OriginAllowed(testBaseHost))
	suite.Require().True(suite.cors.OriginAllowed("http://mynaipi.com:80"))
	suite.Require().True(suite.cors.OriginAllowed("http://mynaipi.com"))
	suite.Require().True(suite.cors.OriginAllowed("mynaipi.com"))
	suite.Require().False(suite.cors.OriginAllowed(dummyHostB))
}

func (suite *TestSuite) TestMethodsAllowed() {

	// regardless to allow origin, empty method is against CORS RFC and should not be treated
	suite.Require().False(suite.cors.MethodAllowed(""))

	// always allow preflight method (e.g.: OPTIONS)
	suite.Require().True(suite.cors.MethodAllowed(suite.cors.PreflightRequestMethod))

	for _, method := range suite.cors.AllowMethods {
		suite.Require().True(suite.cors.MethodAllowed(method))
	}

	// not enabled by default
	suite.Require().False(suite.cors.MethodAllowed(http.MethodTrace))
}

func (suite *TestSuite) TestHeadersAllowed() {
	dummyHeader := "Dummy-Header"

	// allow default headers
	suite.Require().True(suite.cors.HeadersAllowed(suite.cors.AllowHeaders))

	// dummyHeader should be denied at this point
	suite.Require().False(suite.cors.HeadersAllowed([]string{dummyHeader}))

	// add dummyHeader to allowed headers
	suite.cors.AllowHeaders = append(suite.cors.AllowHeaders, dummyHeader)

	// ensure dummyHeader header is allowed
	suite.Require().True(suite.cors.HeadersAllowed([]string{dummyHeader}))
}

func TestCorsSuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}
