package playback

import (
	"context"

	"google.golang.org/grpc"
)

func NewInterceptor(ctx context.Context) grpc.UnaryServerInterceptor {
	playback := FromContext(ctx)
	return func(ctx context.Context, request interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		ctx = NewContext(ctx, playback)

		return handler(ctx, request)
	}
}
