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

package inlineparser

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/nuclio/logger"
	"gopkg.in/yaml.v2"
)

const StartBlockKeyword = "@nuclio."

// ConfigParser parsers inline configuration in files
type ConfigParser interface {
	Parse(path string) (map[string]map[string]interface{}, error)
}

// InlineParser parses comment in code
type InlineParser struct {
	logger                  logger.Logger
	currentStateLineHandler func(line string) error
	currentBlockName        string
	currentBlockContents    string
	currentCommentChar      string
	startBlockPattern       string
	currentBlocks           map[string]map[string]interface{}
}

func NewParser(parentLogger logger.Logger, commentChar string) *InlineParser {
	return &InlineParser{
		logger:             parentLogger.GetChild("inlineparser"),
		currentCommentChar: commentChar,
	}
}

// Parse looks for a block starting with a comment character and "@nuclio.". It then adds this
// to the list of inline configuration blocks. For example
//
// @nuclio.configure
//
// function.yaml:
//   apiVersion: "nuclio.io/v1"
//   kind: "Function"
//   spec:
//     runtime: "golang"
//     triggers:
//       http:
//         maxWorkers: 8
//         kind: http
//
func (p *InlineParser) Parse(path string) (map[string]map[string]interface{}, error) {
	reader, err := os.OpenFile(path, os.O_RDONLY, os.FileMode(0644))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to open function file")
	}
	scanner := bufio.NewScanner(reader)

	// prepare stuff for states
	p.currentBlocks = map[string]map[string]interface{}{}
	p.startBlockPattern = fmt.Sprintf("%s%s", p.currentCommentChar, StartBlockKeyword)

	// init state to looking for start block
	p.currentStateLineHandler = p.lookingForStartBlockStateHandleLine

	p.logger.DebugWith("Starting to look for block pattern", "pattern", p.startBlockPattern)

	// read a line
	for scanner.Scan() {

		// handle the current line in the state machine
		if err := p.currentStateLineHandler(scanner.Text()); err != nil {
			return nil, errors.Wrap(err, "Failed to handle line")
		}
	}

	return p.currentBlocks, nil
}

func (p *InlineParser) lookingForStartBlockStateHandleLine(line string) error {
	spacelessLine := strings.Replace(line, " ", "", -1)

	// if the string starts with <commandChar><space>@nuclio. - we found a match
	if strings.HasPrefix(spacelessLine, p.startBlockPattern) {

		// set current block name: `// @nuclio.createFiles` -> `createFiles`
		p.currentBlockName = strings.Trim(spacelessLine[len(p.startBlockPattern):], " ")
		p.logger.DebugWith("Found block start", "block name", p.currentBlockName)

		// switch state
		p.currentStateLineHandler = p.readingBlockStateHandleLine
	}

	return nil
}

func (p *InlineParser) readingBlockStateHandleLine(line string) error {

	// if the line doesn't start with a comment character, close the block
	if !strings.HasPrefix(line, p.currentCommentChar) {
		unmarshalledBlock := map[string]interface{}{}

		p.logger.DebugWith("Found block end", "contentsLen", len(p.currentBlockContents))

		// parse yaml
		if err := yaml.Unmarshal([]byte(p.currentBlockContents), &unmarshalledBlock); err != nil {
			return errors.Wrapf(err, "Failed to unmarshal inline block: %s", p.currentBlockName)
		}

		// add block to current blocks
		p.currentBlocks[p.currentBlockName] = unmarshalledBlock

		// clear current block
		p.currentBlockContents = ""

		// go back to looking for blocks
		p.currentStateLineHandler = p.lookingForStartBlockStateHandleLine

		// and we're done
		return nil
	}

	// skip the comment
	line = line[len(p.currentCommentChar):]

	// if there's more contents, skip the first space (since space must follow character)
	if len(line) != 0 {
		line = line[1:]
	}

	p.currentBlockContents += line + "\n"

	return nil
}
