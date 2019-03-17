package playback

import (
	"context"
)

var contextKey = struct{}{}

func NewContext(ctx context.Context, p *Playback) context.Context {
	return context.WithValue(ctx, contextKey, p)
}

func FromContext(ctx context.Context) *Playback {
	p, ok := ctx.Value(contextKey).(*Playback)
	if !ok {
		return Default()
	}

	return p
}
