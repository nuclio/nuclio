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
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/errors"
)

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

// RetryUntilSuccessful calls callback every interval for duration until it returns true
func RetryUntilSuccessful(duration time.Duration, interval time.Duration, callback func() bool) error {
	deadline := time.Now().Add(duration)

	// while we haven't passed the deadline
	for !time.Now().After(deadline) {

		// if callback returns true, we're done
		if callback() {
			return nil
		}

		time.Sleep(interval)
	}

	return errors.New("Timed out waiting until successful")
}

// RunningInContainer returns true if currently running in a container, false otherwise
func RunningInContainer() bool {
	return FileExists("/.dockerenv")
}

func Redact(redactions []string, runOutput string) string {
	if redactions == nil {
		return runOutput
	}

	var replacements []string

	for _, redactionField := range redactions {
		replacements = append(replacements, redactionField, "[redacted]")
	}

	replacer := strings.NewReplacer(replacements...)
	return replacer.Replace(runOutput)
}

func StripPrefixes(input string, prefixes []string) string {
	for _, prefix := range prefixes {
		if strings.HasPrefix(input, prefix) {
			return strings.TrimPrefix(input, prefix)
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
