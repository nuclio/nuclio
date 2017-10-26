package command

import "strings"

type stringSliceFlag []string

func (ssf *stringSliceFlag) String() string {
	return strings.Join(*ssf, ", ")
}

func (ssf *stringSliceFlag) Set(value string) error {
	*ssf = append(*ssf, value)
	return nil
}

func (ssf *stringSliceFlag) Type() string {
	return "String"
}
