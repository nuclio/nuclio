package common

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type IsFileTestSuite struct {
	suite.Suite
	tempDir string
}

func (suite *IsFileTestSuite) SetupSuite() {
	var err error

	// Create temp dir for IsFileTestSuite
	suite.tempDir, err = ioutil.TempDir("", "isfile-test")
	suite.Require().NoError(err)
}

func (suite *IsDirTestSuite) TearDownSuite() {
	defer os.RemoveAll(suite.tempDir)
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

	// Set path to unexisted file
	tempFile := filepath.Join(suite.tempDir, "somePath.txt")

	// Verify that function isFile() returns false when file doesn't exist in the system
	suite.Require().False(IsFile(tempFile))
}

func (suite *IsFileTestSuite) TestFileIsADirectory() {
	suite.Require().False(IsFile(suite.tempDir))
}

type IsDirTestSuite struct {
	suite.Suite
	tempDir string
}

func (suite *IsDirTestSuite) SetupSuite() {
	var err error

	// Create temp dir for IsDirTestSuite
	suite.tempDir, err = ioutil.TempDir("", "isdir-test")
	suite.Require().NoError(err)
}

func (suite *IsFileTestSuite) TearDownSuite() {
	defer os.RemoveAll(suite.tempDir)
}

func (suite *IsDirTestSuite) TestPositive() {

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

type FileExistTestSuite struct {
	suite.Suite
	tempDir string
}

func (suite *FileExistTestSuite) SetupSuite() {
	var err error

	// Create temp dir for FileExistTestSuite
	suite.tempDir, err = ioutil.TempDir("", "file_exists-test")
	suite.Require().NoError(err)
}

func (suite *FileExistTestSuite) TearDownSuite() {
	defer os.RemoveAll(suite.tempDir)
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

	// Set path to unexisted file
	tempFile := filepath.Join(suite.tempDir, "somePath.txt")

	// Verify that function FileExists() returns false when file doesn't exist
	suite.Require().False(FileExists(tempFile))
}

func (suite *FileExistTestSuite) TestFileIsNotAFile() {

	// Verify that function returns true when folder is exist in the system
	suite.Require().True(FileExists(suite.tempDir))
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

type RetryUntilSuccessfulTestSuite struct {
	suite.Suite
}

func (suite *RetryUntilSuccessfulTestSuite) TestPositive() {
	err := RetryUntilSuccessful(50*time.Millisecond, 10*time.Millisecond, func() bool {
		return true
	})

	suite.Require().NoError(err)
}

func (suite *RetryUntilSuccessfulTestSuite) TestNegative() {
	err := RetryUntilSuccessful(50*time.Millisecond, 10*time.Millisecond, func() bool {
		return false
	})

	suite.Require().Error(err)

}

func (suite *RetryUntilSuccessfulTestSuite) TestNumberOfCalls() {

	// Create actual and expected number of calls
	actualNumberOfCalls := 0
	expectedNumberOfCalls := 10

	_ = RetryUntilSuccessful(1000*time.Millisecond, 100*time.Millisecond, func() bool {
		_, _, _, ok := runtime.Caller(1)
		if ok {
			actualNumberOfCalls++
		}
		return false
	})

	suite.Require().Equal(expectedNumberOfCalls, actualNumberOfCalls)
}

func (suite *RetryUntilSuccessfulTestSuite) TestTimeBetweenIntervals() {

	// Starting time from currentTime - 100ms cause function calls right now
	startingIntervalTime := getCurrentTimeInMilliseconds() - 100
	_ = RetryUntilSuccessful(1000*time.Millisecond, 100*time.Millisecond, func() bool {
		_, _, _, ok := runtime.Caller(1)
		if ok {

			// If call was successfull create finishIntervalTime variable and set currentTime
			finishIntervalTime := getCurrentTimeInMilliseconds()

			// Verify that difference between previous interval and current interval is from 60 to 120ms
			suite.Require().True((finishIntervalTime-startingIntervalTime > 60) && (finishIntervalTime-startingIntervalTime < 120))

			// Set currentInterval time value into previous interval variable
			startingIntervalTime = finishIntervalTime
		}
		return false
	})
}

func (suite *RetryUntilSuccessfulTestSuite) TestDurationTime() {

	// Initialize startTime as currentTime
	startTime := getCurrentTimeInMilliseconds()
	_ = RetryUntilSuccessful(1000*time.Millisecond, 100*time.Millisecond, func() bool {
		return false
	})

	// Initialize finishTime as currentTime
	finishTime := getCurrentTimeInMilliseconds()

	// Verify that function duration is as expected
	suite.Require().True((finishTime-startTime > 960) && (finishTime-startTime < 1060))
}

func getCurrentTimeInMilliseconds() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

type StripPrefixesTestSuite struct {
	suite.Suite
}

func (suite *StripPrefixesTestSuite) TestPositive() {
	stripped := StripPrefixes("prefix_something_1", []string{"prefix_"})
	suite.Require().Equal("something_1", stripped)

	stripped = StripPrefixes("prefix_something_1", []string{"not_prefix", "prefix_"})
	suite.Require().Equal("something_1", stripped)

	stripped = StripPrefixes("prefix_something_1", []string{"prefix_", "not_prefix", "not_prefix_2"})
	suite.Require().Equal("something_1", stripped)

	stripped = StripPrefixes("prefix_something_1", []string{"not_prefix", "not_prefix_2"})
	suite.Require().Equal("prefix_something_1", stripped)
}

func TestHelperTestSuite(t *testing.T) {
	suite.Run(t, new(RetryUntilSuccessfulTestSuite))
	suite.Run(t, new(StringSliceToIntSliceTestSuite))
	suite.Run(t, new(FileExistTestSuite))
	suite.Run(t, new(IsDirTestSuite))
	suite.Run(t, new(IsFileTestSuite))
	suite.Run(t, new(StripPrefixesTestSuite))
}
