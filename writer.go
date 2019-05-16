package playback

import (
	"io"
	"os"
)

type Writer interface {
	io.WriteCloser
	Sync() error
	Name() string
	Type() PathType
}

type PathType string

const (
	PathTypeNil  = PathType("")
	PathTypeFile = PathType("writer")
)

type file struct {
	*os.File
}

func (f file) Type() PathType {
	return PathTypeFile
}
