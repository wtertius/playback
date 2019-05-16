package playback

import (
	"context"

	"google.golang.org/grpc"
)

func NewInterceptor(ctx context.Context) grpc.UnaryServerInterceptor {
	cassette := FromContext(ctx)
	return func(ctx context.Context, request interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		ctx = cassette.playback.NewContext(ctx)

		return handler(ctx, request)
	}
}
