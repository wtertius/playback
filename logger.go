package playback

import (
	"fmt"
)

type Logger interface {
	Debugf(format string, args ...interface{})
}

type defaultLogger struct{}

func (l *defaultLogger) Debugf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}
