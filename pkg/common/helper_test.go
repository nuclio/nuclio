package common

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type IsFileTestSuite struct {
	suite.Suite
	tempDir string
}

type IsDirTestSuite struct {
	suite.Suite
	tempDir string
}

type FileExistTestSuite struct {
	suite.Suite
	tempDir string
}

type StringSliceToIntSliceTestSuite struct {
	suite.Suite
}

type RetryUntilSuccessfulTestSuite struct {
	suite.Suite
}

func (suite *IsFileTestSuite) SetupTest()  {
	var err error

	// create a temp directory
	suite.tempDir, err = ioutil.TempDir("", "isfile-test")
	suite.Require().NoError(err)
}

func (suite *IsFileTestSuite) TestIsFileFuncPositive() {
	tempFile, err := ioutil.TempFile(suite.tempDir, "temp_file")
	suite.Require().NoError(err)
	suite.Assert().True(IsFile(tempFile.Name()))
	defer os.Remove(tempFile.Name())
}

func (suite *IsFileTestSuite) TestIsFileFuncFileIsNotExist() {
	tempFile, err := ioutil.TempFile(suite.tempDir, "temp_file")
	suite.Require().NoError(err)
	os.Remove(tempFile.Name())
	suite.Assert().False(IsFile(tempFile.Name()))
}

func (suite *IsFileTestSuite) TestIsFileFuncFileIsADirectory() {
	suite.Assert().False(IsFile(suite.tempDir))
}

func (suite *IsFileTestSuite) TearDown() {
	defer os.RemoveAll(suite.tempDir)
}

func (suite *IsDirTestSuite) TestIsDirFuncPositive() {
	var err error

	// create a temp directory
	suite.tempDir, err = ioutil.TempDir("", "isdir-test")
	suite.Require().NoError(err)
	suite.Assert().True(IsDir(suite.tempDir))
}

func (suite *IsDirTestSuite) TestIsDirFuncNegative() {
	tempFile, err := ioutil.TempFile(suite.tempDir, "temp_file")
	suite.Require().NoError(err)
	suite.Assert().False(IsDir(tempFile.Name()))
	defer os.Remove(tempFile.Name())
}

func (suite *IsDirTestSuite) TearDown() {
	defer os.RemoveAll(suite.tempDir)
}

func (suite *FileExistTestSuite) SetupTest()  {
	var err error

	// create a temp directory
	suite.tempDir, err = ioutil.TempDir("", "file_exist-test")
	suite.Require().NoError(err)
}

func (suite *FileExistTestSuite) TestFileExistsFuncPositive() {
	tempFile, err := ioutil.TempFile(suite.tempDir, "temp_file")
	suite.Require().NoError(err)
	suite.Assert().True(IsFile(tempFile.Name()))
	defer os.Remove(tempFile.Name())
}

func (suite *FileExistTestSuite) TestFileNotExist() {
	tempFile, err := ioutil.TempFile(suite.tempDir, "temp_file")
	suite.Require().NoError(err)
	os.Remove(tempFile.Name())
	suite.Assert().False(IsFile(tempFile.Name()))
}

func (suite *FileExistTestSuite) TestFileExistFuncFileIsNotAFile() {
	suite.Assert().False(IsFile(suite.tempDir))
}

func (suite *FileExistTestSuite) TearDown() {
	defer os.RemoveAll(suite.tempDir)
}

func (suite *StringSliceToIntSliceTestSuite) TestStringSliceToIntSlicePositive() {
	stringSlice := []string{"1","2","5","6","23"}
	expectedSlice := []int{1,2,5,6,23}
	actualSlice, err := StringSliceToIntSlice(stringSlice);
	suite.Assert().Equal(expectedSlice,actualSlice)
	suite.Assert().Equal(nil,err)
}

func (suite *StringSliceToIntSliceTestSuite) TestStringSliceToIntSliceNegativeData() {
	stringSlice := []string{"1","2","5","6","23", "someBadData"}
	_, err := StringSliceToIntSlice(stringSlice);
	suite.Assert().True(err!=nil)
}

func (suite *RetryUntilSuccessfulTestSuite) TestRetryUntilSuccessfulPositive() {
	err:= RetryUntilSuccessful(2*time.Second, 1*time.Second, func() bool {
		return true
	})
	suite.Assert().True(err==nil)
}

func (suite *RetryUntilSuccessfulTestSuite) TestRetryUntilSuccessfulNegative() {
	err:= RetryUntilSuccessful(2*time.Second, 1*time.Second, func() bool {
		return false
	})
	suite.Assert().True(err!=nil)

}

func TestIsFileTestSuite(t *testing.T) {
	suite.Run(t, new(IsFileTestSuite))
}

func TestIsDirTestSuite(t *testing.T) {
	suite.Run(t, new(IsDirTestSuite))
}

func TestFileExistsTestSuite(t *testing.T) {
	suite.Run(t, new(FileExistTestSuite))
}

func TestStringSliceToIntSliceTestSuite(t *testing.T) {
	suite.Run(t, new(StringSliceToIntSliceTestSuite))
}

func TestRetryUntilSuccessfulTestSuite(t *testing.T) {
	suite.Run(t, new(RetryUntilSuccessfulTestSuite))
}