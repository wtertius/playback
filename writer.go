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
	ReadOnly() bool
}

type PathType string

const (
	PathTypeNil  = PathType("")
	PathTypeFile = PathType("file")
)

type file struct {
	*os.File
}

func (f *file) Type() PathType {
	return PathTypeFile
}

func (*file) ReadOnly() bool {
	return false
}

type nilNamed struct {
	typ  PathType
	name string
}

func newNilNamed(typ PathType, name string) *nilNamed {
	return &nilNamed{
		typ:  typ,
		name: name,
	}
}

func (n *nilNamed) Type() PathType {
	return n.typ
}

func (n *nilNamed) Name() string {
	return n.name
}

func (*nilNamed) ReadOnly() bool {
	return true
}

func (*nilNamed) Close() error {
	return nil
}

func (*nilNamed) Sync() error {
	return nil
}

func (*nilNamed) Write([]byte) (int, error) {
	return 0, nil
}
