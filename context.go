package playback

import (
	"context"
)

var contextKeyCassette = struct{}{}

func FromContext(ctx context.Context) *Playback {
	c := CassetteFromContext(ctx)
	if c == nil {
		return nil
	}

	return c.playback
}

func ProxyCassetteContext(ctx context.Context) context.Context {
	ctx = NewContextWithCassette(ctx, CassetteFromContext(ctx))
	return ctx
}

func CassetteFromContext(ctx context.Context) *Cassette {
	c, _ := ctx.Value(contextKeyCassette).(*Cassette)

	return c
}

func (p *Playback) NewContext(ctx context.Context) context.Context {
	c, err := p.NewCassette()
	if err != nil {
		p.Error = err
	}

	return context.WithValue(ctx, contextKeyCassette, c)
}

func NewContextWithCassette(ctx context.Context, cassette *Cassette) context.Context {
	return context.WithValue(ctx, contextKeyCassette, cassette)
}
