package util

import (
	"github.com/nuclio/nuclio-sdk"

	"github.com/mholt/archiver"
	"github.com/nuclio/nuclio/pkg/errors"
	"fmt"
	"reflect"
)

type Decompressor struct {
	logger nuclio.Logger
}

func NewDecompressor(parentLogger nuclio.Logger) (*Decompressor) {
	newDecompressor := &Decompressor{
		logger: parentLogger,
	}

	return newDecompressor
}

func (d *Decompressor) Decompress(source string, target string) error {
	fileArchiver := archiver.MatchingFormat(source)
	if fileArchiver == nil {
		return errors.New(fmt.Sprintf("File %s is not compressed or has an unknown extension", source))
	}

	d.logger.DebugWith("File is compressed, now decompressing",
		"file", source,
		"compression", reflect.TypeOf(fileArchiver),
		"target", target)

	if err := fileArchiver.Open(source, target); err != nil {
		return errors.Wrapf(err, "Failed to decompress file %s", source)
	}

	return nil
}

func IsCompressed(source string) bool {
	fileArchiver := archiver.MatchingFormat(source)
	return fileArchiver != nil
}
