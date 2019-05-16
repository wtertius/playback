package playback

import (
	"context"
)

var contextKey = struct{}{}

func FromContext(ctx context.Context) *Cassette {
	c, ok := ctx.Value(contextKey).(*Cassette)
	if !ok {
		c, _ = Default().NewCassette()
	}

	return c
}
