package common

import (
	"io/ioutil"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type IsFileTestSuite struct {
	suite.Suite
	tempDir string
}

func (suite *IsFileTestSuite) TestPositive() {
	// Create temp file
	tempFile, err := ioutil.TempFile(suite.tempDir, "temp_file")
	suite.Require().NoError(err)
	defer os.Remove(tempFile.Name())

	// Verify that function isFile() returns true when file is created
	suite.Require().True(IsFile(tempFile.Name()))

}

func (suite *IsFileTestSuite) TestFileIsNotExist() {
	// Create temp file
	tempFile, err := ioutil.TempFile(suite.tempDir, "temp_file")
	suite.Require().NoError(err)
	os.Remove(tempFile.Name())

	// Verify that function isFile() returns false when file doesn't exist in the system
	suite.Require().False(IsFile(tempFile.Name()))
}

func (suite *IsFileTestSuite) TestFileIsADirectory() {
	var err error

	// Create temp directory
	suite.tempDir, err = ioutil.TempDir("", "isfile-test")
	suite.Require().NoError(err)
	defer os.RemoveAll(suite.tempDir)

	suite.Require().False(IsFile(suite.tempDir))

	// Set up temp dir in empty string to prevent no such file or directory
	// When next functions will creates file from /tmp/path_to_dir/path_to_file_which_func_want_to_create
	suite.tempDir = ""
}

func TestIsFileTestSuite(t *testing.T) {
	suite.Run(t, new(IsFileTestSuite))
}

type IsDirTestSuite struct {
	suite.Suite
	tempDir string
}

func (suite *IsDirTestSuite) TestPositive() {
	var err error

	// Create a temp directory
	suite.tempDir, err = ioutil.TempDir("", "isdir-test")
	suite.Require().NoError(err)
	defer os.RemoveAll(suite.tempDir)

	// Verify that function IsDir() returns true when directory exists in the system
	suite.Require().True(IsDir(suite.tempDir))
}

func (suite *IsDirTestSuite) TestNegative() {
	// Create temp file
	tempFile, err := ioutil.TempFile(suite.tempDir, "temp_file")
	suite.Require().NoError(err)
	defer os.Remove(tempFile.Name())

	// Verify that function IsDir( returns false when file instead of directory is function argument
	suite.Require().False(IsDir(tempFile.Name()))
}

func TestIsDirTestSuite(t *testing.T) {
	suite.Run(t, new(IsDirTestSuite))
}

type FileExistTestSuite struct {
	suite.Suite
	tempDir string
}

func (suite *FileExistTestSuite) TestPositive() {
	// Create temp file
	tempFile, err := ioutil.TempFile(suite.tempDir, "temp_file")
	suite.Require().NoError(err)
	defer os.Remove(tempFile.Name())

	// Verify that function FileExists() returns true when file is exist
	suite.Require().True(FileExists(tempFile.Name()))
}

func (suite *FileExistTestSuite) TestFileNotExist() {
	// Create temp file
	tempFile, err := ioutil.TempFile(suite.tempDir, "temp_file")
	suite.Require().NoError(err)
	os.Remove(tempFile.Name())

	// Verify that function FileExists() returns false when file doesn't exist
	suite.Require().False(FileExists(tempFile.Name()))
}

func (suite *FileExistTestSuite) TestFileIsNotAFile() {
	var err error

	// Create temp directory
	suite.tempDir, err = ioutil.TempDir("", "file_exists-test")
	suite.Require().NoError(err)
	defer os.RemoveAll(suite.tempDir)

	// Verify that function returns
	suite.Require().True(FileExists(suite.tempDir))

	// Set up temp dir in empty string to prevent no such file or directory
	// When next functions will creates file from /tmp/path_to_dir/path_to_file_which_func_want_to_create
	suite.tempDir = ""
}

func TestFileExistsTestSuite(t *testing.T) {
	suite.Run(t, new(FileExistTestSuite))
}

type StringSliceToIntSliceTestSuite struct {
	suite.Suite
}

func (suite *StringSliceToIntSliceTestSuite) TestPositive() {
	// Prepare slice for StringSliceToIntSlice() function
	stringSlice := []string{"1", "2", "5", "6", "23"}
	expectedSlice := []int{1, 2, 5, 6, 23}
	actualSlice, err := StringSliceToIntSlice(stringSlice)

	// Check that slice successfully casted into []int slice
	suite.Require().NoError(err)
	suite.Require().Equal(expectedSlice, actualSlice)
}

func (suite *StringSliceToIntSliceTestSuite) TestNegativeData() {
	// Prepare incorrect (for casting) slice for StringSliceToIntSlice() function
	stringSlice := []string{"1", "2", "5", "6", "23", "someBadData"}
	_, err := StringSliceToIntSlice(stringSlice)

	// Verify that error is throws by StringSliceToIntSlice() function
	suite.Require().Error(err)
}

func TestStringSliceToIntSliceTestSuite(t *testing.T) {
	suite.Run(t, new(StringSliceToIntSliceTestSuite))
}

type RetryUntilSuccessfulTestSuite struct {
	suite.Suite
}

func (suite *RetryUntilSuccessfulTestSuite) TestPositive() {
	err := RetryUntilSuccessful(2*time.Second, 1*time.Second, func() bool {
		return true
	})

	suite.Require().NoError(err)
}

func (suite *RetryUntilSuccessfulTestSuite) TestNegative() {
	err := RetryUntilSuccessful(2*time.Second, 1*time.Second, func() bool {
		return false
	})

	suite.Require().Error(err)

}

func (suite *RetryUntilSuccessfulTestSuite) TestNumberOfCalls() {
	// Create actual and expected number of calls
	actualNumberOfCalls := 0
	expectedNumberOfCalls := 15

	_ = RetryUntilSuccessful(15*time.Second, 1*time.Second, func() bool {
		_, _, _, ok := runtime.Caller(1)
		if ok {
			actualNumberOfCalls++
		}
		return false
	})

	suite.Require().Equal(expectedNumberOfCalls, actualNumberOfCalls)
}

func (suite *RetryUntilSuccessfulTestSuite) TestTimeBetweenIntervals() {
	// Starting time from currentTime - 1 cause function calls right now
	startingIntervalTime := time.Now().Unix() - 1
	_ = RetryUntilSuccessful(55*time.Second, 1*time.Second, func() bool {
		_, _, _, ok := runtime.Caller(1)
		if ok {
			// If call was successfull create finishIntervalTime variable and set currentTime
			finishIntervalTime := time.Now().Unix()
			// Verify that difference betwen previous interval and current interval is 1
			suite.Require().True(finishIntervalTime-startingIntervalTime == 1)
			// Set currentInterval time value into previos interval variable
			startingIntervalTime = finishIntervalTime
		}
		return false
	})

}

func (suite *RetryUntilSuccessfulTestSuite) TestDurationTime() {
	// Initialize startTime as currentTime
	startTime := time.Now().Unix()
	_ = RetryUntilSuccessful(10*time.Second, 1*time.Second, func() bool {
		return false
	})
	// Initialize finishTime as currentTime
	finishTime := time.Now().Unix()

	// Verify that function duration is as expected
	suite.Require().True(finishTime-startTime == 10)
}

func TestRetryUntilSuccessfulTestSuite(t *testing.T) {
	suite.Run(t, new(RetryUntilSuccessfulTestSuite))
}
