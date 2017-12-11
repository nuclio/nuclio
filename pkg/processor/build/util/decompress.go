package util

import (
	"fmt"
	"reflect"

	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/mholt/archiver"
	"github.com/nuclio/nuclio-sdk"
)

type Decompressor struct {
	logger nuclio.Logger
}

func NewDecompressor(parentLogger nuclio.Logger) (*Decompressor, error) {
	newDecompressor := &Decompressor{
		logger: parentLogger,
	}

	return newDecompressor, nil
}

func (d *Decompressor) Decompress(source string, target string) error {
	fileArchiver := archiver.MatchingFormat(source)
	if fileArchiver == nil {
		return fmt.Errorf("File %s is not compressed or has an unknown extension", source)
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
