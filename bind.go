package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

// bindSpec is one -bind or -robind option.
type bindSpec struct {
	src      string
	dest     string
	readOnly bool
}

// bindFlag collects repeated -bind and -robind options into one ordered list, so
// that the order given on the command line is preserved between the two.
type bindFlag struct {
	specs    *[]bindSpec
	readOnly bool
}

func (f *bindFlag) String() string {
	return ""
}

func (f *bindFlag) Set(value string) error {
	spec, err := parseBindSpec(value, f.readOnly)
	if err != nil {
		return err
	}

	*f.specs = append(*f.specs, spec)

	return nil
}

// parseBindSpec parses a "src[:dest]" option value. Both paths have to be
// absolute, and dest defaults to src.
func parseBindSpec(value string, readOnly bool) (bindSpec, error) {
	src, dest, found := strings.Cut(value, ":")
	if !found {
		dest = src
	}

	if !strings.HasPrefix(src, "/") || !strings.HasPrefix(dest, "/") {
		return bindSpec{}, fmt.Errorf("paths of src[:dest] must be absolute: %s", value)
	}

	return bindSpec{
		src:      filepath.Clean(src),
		dest:     filepath.Clean(dest),
		readOnly: readOnly,
	}, nil
}
