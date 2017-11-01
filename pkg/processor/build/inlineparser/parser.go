package inlineparser

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/nuclio/nuclio-sdk"
	"gopkg.in/yaml.v2"
)

type Parser struct {
	logger                  nuclio.Logger
	currentStateLineHandler func(line string) error
	currentBlockName        string
	currentBlockContents    string
	currentCommentChar      string
	startBlockPattern       string
	currentBlocks           map[string]map[string]interface{}
}

func NewParser(parentLogger nuclio.Logger) (*Parser, error) {
	return &Parser{
		logger: parentLogger.GetChild("inlineparser"),
	}, nil
}

func (p *Parser) Parse(reader io.Reader, commentChar string) (map[string]map[string]interface{}, error) {
	scanner := bufio.NewScanner(reader)

	// prepare stuff for states
	p.currentBlocks = map[string]map[string]interface{}{}
	p.currentCommentChar = commentChar
	p.startBlockPattern = fmt.Sprintf("%s @nuclio.", commentChar)

	// init state to looking for start block
	p.currentStateLineHandler = p.lookingForStartBlockStateHandleLine

	// read a line
	for scanner.Scan() {

		// handle the current line in the state machine
		if err := p.currentStateLineHandler(scanner.Text()); err != nil {
			return nil, errors.Wrap(err, "Failed to handle line")
		}
	}

	return p.currentBlocks, nil
}

func (p *Parser) lookingForStartBlockStateHandleLine(line string) error {

	// if the string starts with <commandChar><space>@nuclio. - we found a match
	if strings.HasPrefix(line, p.startBlockPattern) {

		// set current block name: `// @nuclio.createFiles` -> `createFiles`
		p.currentBlockName = line[len(p.startBlockPattern):]

		// switch state
		p.currentStateLineHandler = p.readingBlockStateHandleLine
	}

	return nil
}

func (p *Parser) readingBlockStateHandleLine(line string) error {

	// if the line doesn't start with a comment character, close the block
	if !strings.HasPrefix(line, p.currentCommentChar) {
		unmarshalledBlock := map[string]interface{}{}

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
