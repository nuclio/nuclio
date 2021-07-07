/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"text/template"
	"time"
	"unicode/utf8"
	"unsafe"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var SmallLettersAndNumbers = []rune("abcdefghijklmnopqrstuvwxyz1234567890")
var LettersAndNumbers = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890")
var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))

// IsFile returns true if the object @ path is a file
func IsFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}

// IsDir returns true if the object @ path is a dir
func IsDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return info.IsDir()
}

// FileExists returns true if the file @ path exists
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// StringSliceToIntSlice converts slices of strings to slices of int. e.g. ["1", "3"] -> [1, 3]
func StringSliceToIntSlice(stringSlice []string) ([]int, error) {
	result := []int{}

	for _, stringValue := range stringSlice {
		var intValue int
		var err error

		if intValue, err = strconv.Atoi(stringValue); err != nil {
			return nil, err
		}

		result = append(result, intValue)
	}

	return result, nil
}

// returns whether the input str is in the slice
func StringSliceContainsString(slice []string, str string) bool {
	for _, stringInSlice := range slice {
		if stringInSlice == str {
			return true
		}
	}

	return false
}

// returns whether the input str has prefix
func StringSliceContainsStringPrefix(prefixes []string, str string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(str, prefix) {
			return true
		}
	}
	return false
}

// returns whether the input str is in the slice case-insensitive
func StringSliceContainsStringCaseInsensitive(slice []string, str string) bool {
	for _, stringInSlice := range slice {
		if strings.EqualFold(stringInSlice, str) {
			return true
		}
	}

	return false
}

// strips out ANSI Colors chars from string
// example: "\u001b[31mHelloWorld" -> "HelloWorld"
func RemoveANSIColorsFromString(s string) string {
	ansi := "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"
	re := regexp.MustCompile(ansi)

	return re.ReplaceAllString(s, "")
}

// RetryUntilSuccessful calls callback every interval for duration until it returns true
func RetryUntilSuccessful(duration time.Duration, interval time.Duration, callback func() bool) error {
	return retryUntilSuccessful(duration, interval, func() (bool, error) {

		// callback results indicate whether to retry
		return !callback(), nil
	})
}

// RetryUntilSuccessfulOnErrorPatterns calls callback every interval for duration as long as error pattern is matched
func RetryUntilSuccessfulOnErrorPatterns(duration time.Duration,
	interval time.Duration,
	errorRegexPatterns []string,
	callback func() string) error {

	return retryUntilSuccessful(duration, interval, func() (bool, error) {
		callbackErrorStr := callback()
		if callbackErrorStr == "" {

			// no error message means no error, succeeded
			return false, nil
		}

		// find a matching error pattern
		if !MatchStringPatterns(errorRegexPatterns, callbackErrorStr) {

			// no error pattern found, dont retry, bail
			return false, errors.Errorf("Failed matching an error pattern for callback: %s", callbackErrorStr)
		}

		return true, nil

	})
}

// retryUntilSuccessful calls callback every interval until duration as long as it should retry
func retryUntilSuccessful(duration time.Duration,
	interval time.Duration,
	callback func() (bool, error)) error {
	var lastErr error
	timedOutErrorMessage := "Timed out waiting until successful"
	deadline := time.Now().Add(duration)

	// while we haven't passed the deadline
	for !time.Now().After(deadline) {
		shouldRetry, err := callback()
		lastErr = err
		if !shouldRetry {
			return err
		}
		time.Sleep(interval)
		continue

	}
	if lastErr != nil {

		// wrap last error
		return errors.Wrapf(lastErr, timedOutErrorMessage)
	}

	// duration expired without any last error
	return errors.Errorf(timedOutErrorMessage)
}

// RunningInContainer returns true if currently running in a container, false otherwise
func RunningInContainer() bool {
	return FileExists("/.dockerenv")
}

func StripPrefixes(input string, prefixes []string) string {
	for _, prefix := range prefixes {
		if strings.HasPrefix(input, prefix) {
			return strings.TrimPrefix(input, prefix)
		}
	}
	return input
}

func StripSuffixes(input string, suffixes []string) string {
	for _, suffix := range suffixes {
		if strings.HasSuffix(input, suffix) {
			return strings.TrimSuffix(input, suffix)
		}
	}
	return input
}

// RemoveEmptyLines removes all empty lines from a string
func RemoveEmptyLines(input string) string {
	var nonEmptyLines []string

	scanner := bufio.NewScanner(strings.NewReader(input))

	// iterate over input line by line. if the line is not empty, shove it to the list
	for scanner.Scan() {
		line := scanner.Text()

		if len(line) != 0 {
			nonEmptyLines = append(nonEmptyLines, line)
		}
	}

	// join the strings with a newline between them
	return strings.Join(nonEmptyLines, "\n")
}

// Generate a function that returns whether a given string matches the specified string
func GenerateStringMatchVerifier(str string) func(string) bool {
	return func(toMatch string) bool {
		return toMatch == str
	}
}

// Removing windows carriage character '\r' when it follows by '\n'
func RemoveWindowsCarriage(b []byte) []byte {
	n := utf8.RuneCount(b)
	for i := 0; i < n-1; i++ {
		if b[i] == '\r' && b[i+1] == '\n' {

			// remove \r, keep \n
			b = append(b[:i], b[i+1:]...)
			n--
		}
	}
	return b
}

func FixEscapeChars(s string) string {
	escapeCharsMap := map[string]string{
		"\\n":  "\n",
		"\\t":  "\t",
		"\\\\": "\\",
		"\\\"": "\"",
	}

	for oldChar, newChar := range escapeCharsMap {
		s = strings.ReplaceAll(s, oldChar, newChar)
	}

	return s
}

func GetEnvOrDefaultString(key string, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	} else if value == "nil" || value == "none" {
		return ""
	}
	return value
}

func GetEnvOrDefaultBool(key string, defaultValue bool) bool {
	return strings.ToLower(GetEnvOrDefaultString(key, strconv.FormatBool(defaultValue))) == "true"
}

// Checks if the given @dirPath is in a java project structure
// for example if the following dir existed "/my-project/src/main/java" then IsJavaProjectDir("/my-project") -> true
func IsJavaProjectDir(dirPath string) bool {
	javaProjectStructurePath := path.Join(dirPath, "src", "main", "java")
	if _, err := os.Stat(javaProjectStructurePath); err != nil {
		return false
	}

	return true
}

func RenderTemplate(text string, data map[string]interface{}) (string, error) {
	templateToRender, err := template.New("t").Parse(text)
	if err != nil {
		return "", errors.Wrap(err, "Failed to create template")
	}

	return renderTemplate(templateToRender, data)
}

func RenderTemplateWithCustomDelimiters(text string,
	data map[string]interface{},
	leftDelimiter string,
	rightDelimiter string) (string, error) {

	templateToRender, err := template.New("t").
		Delims(leftDelimiter, rightDelimiter).
		Parse(text)
	if err != nil {
		return "", errors.Wrap(err, "Failed to create template with custom delimiters")
	}

	return renderTemplate(templateToRender, data)
}

func renderTemplate(templateToRender *template.Template, data map[string]interface{}) (string, error) {
	var templateToRenderBuffer bytes.Buffer
	if err := templateToRender.Execute(&templateToRenderBuffer, &data); err != nil {
		return "", errors.Wrap(err, "Failed to execute template rendering")
	}

	return templateToRenderBuffer.String(), nil
}

func GetDurationOrInfinite(timeout *time.Duration) time.Duration {
	if timeout != nil {
		return *timeout
	}

	// essentially infinite
	return 100 * 365 * 24 * time.Hour
}

func GetSourceDir() string {

	// get caller filename
	_, fileName, _, _ := runtime.Caller(0)

	// get filename dir
	dirName := path.Dir(fileName)

	for {

		// we determine source dir by having a `go.mod` file there
		if _, err := os.Stat(filepath.Join(dirName, "go.mod")); !os.IsNotExist(err) {
			return dirName
		}

		// if we didn't find source yet, try on parent dir
		dirName = filepath.Dir(dirName)

		// we're out of parents, stop here
		if dirName == "/" {
			return dirName
		}
	}
}

// Quote returns a shell-escaped version of the string s. The returned value
// is a string that can safely be used as one token in a shell command line.
func Quote(s string) string {
	var specialCharPattern = regexp.MustCompile(`[^\w@%+=:,./-]`)

	if len(s) == 0 {
		return "''"
	}
	if specialCharPattern.MatchString(s) {
		return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
	}

	return s
}

func ByteSliceToString(b []byte) string {

	// https://golang.org/src/strings/builder.go#L45
	// effectively converts bytes to string
	// !! use with caution as returned string is mutable !!
	return *(*string)(unsafe.Pointer(&b))
}

func MatchStringPatterns(patterns []string, s string) bool {
	for _, pattern := range patterns {
		if regexp.MustCompile(pattern).MatchString(s) {

			// one matching pattern is enough
			return true
		}
	}
	return false
}

func CompileImageName(registryURL string, imageName string) string {
	return strings.TrimSuffix(registryURL, "/") + "/" + imageName
}

func AnyPositiveInSliceInt64(numbers []int64) bool {
	for _, number := range numbers {
		if number >= 0 {
			return true
		}
	}
	return false
}

func GenerateRandomString(length int, letters []rune) string {
	randomString := make([]rune, length)
	for i := range randomString {
		randomString[i] = letters[seededRand.Intn(len(letters))]
	}

	return string(randomString)
}

func CatchAndLogPanic(ctx context.Context,
	loggerInstance logger.Logger,
	actionName string) error {
	return CatchAndLogPanicWithOptions(ctx,
		loggerInstance,
		actionName,
		&CatchAndLogPanicOptions{
			Args:          nil,
			CustomHandler: nil,
		})
}

func CatchAndLogPanicWithOptions(ctx context.Context,
	loggerInstance logger.Logger,
	actionName string,
	options *CatchAndLogPanicOptions) error {
	if err := recover(); err != nil {
		callStack := debug.Stack()
		LogPanic(ctx, loggerInstance, actionName, options.Args, callStack, err)

		asErr := ErrorFromRecoveredError(err)
		if options.CustomHandler != nil {
			options.CustomHandler(asErr)
		}
		return asErr
	}
	return nil
}

// LabelsMapMatchByLabelSelector returns whether a labelsMap is matched by an encoded label selector
// corresponding to https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/
//
// Example:
//   labelsMap:
//     a: b
//     c: d
//   encodedLabelSelector: a=b
// returns true
func LabelsMapMatchByLabelSelector(labelSelector string, labelsMap map[string]string) (bool, error) {

	parsedLabelSelector, err := metav1.ParseToLabelSelector(labelSelector)
	if err != nil {
		return false, errors.Wrap(err, "Failed to parse label selector")
	}
	selector, err := metav1.LabelSelectorAsSelector(parsedLabelSelector)
	if err != nil {
		return false, errors.Wrap(err, "Failed to get selector from label selector")
	}
	return selector.Matches(labels.Set(labelsMap)), nil
}

// GetRuntimeNameAndVersion return runtime name and version.
// e.g. go:1.8 -> go, 1.8
func GetRuntimeNameAndVersion(runtime string) (string, string) {

	switch runtimeAndVersion := strings.Split(runtime, ":"); len(runtimeAndVersion) {

	// if both are passed (e.g. python:3.6) - return them both
	case 2:
		return runtimeAndVersion[0], runtimeAndVersion[1]

	// otherwise - return the first element (e.g. go -> go)
	default:
		return runtimeAndVersion[0], ""
	}

}

func LogPanic(ctx context.Context,
	loggerInstance logger.Logger,
	actionName string,
	args []interface{},
	callStack []byte,
	err interface{}) {

	logArgs := []interface{}{
		"err", err,
		"stack", string(callStack),
	}

	if args != nil {
		logArgs = append(logArgs, args...)
	}

	loggerInstance.ErrorWithCtx(ctx, "Panic caught while "+actionName, logArgs...)
}

func ErrorFromRecoveredError(recoveredError interface{}) error {
	switch typedErr := recoveredError.(type) {
	case string:
		return errors.New(typedErr)
	case error:
		return typedErr
	default:
		return errors.New(fmt.Sprintf("Unknown error type: %s", reflect.TypeOf(typedErr)))
	}
}
