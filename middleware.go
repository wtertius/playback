package playback

import (
	"context"
	"fmt"
	"net/http"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	HeaderCassetteID       = "x-playback-id"
	HeaderCassettePathType = "x-playback-path-type"
	HeaderCassettePathName = "x-playback-path-name"
	HeaderMode             = "x-playback-mode"
	HeaderSuccess          = "x-playback-success"
)

func (p *Playback) NewHTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		cassette := p.incomingCassetteFromHTTPRequest(req)

		mode := cassette.Mode()

		ctx := NewContextWithCassette(req.Context(), cassette)
		req = req.WithContext(ctx)

		if mode == ModeRecord {
			cassette.SetHTTPRequest(req)
		}

		rw := multiplexHTTPResponseWriter(w, mode)
		if pathType := cassette.PathType(); pathType != PathTypeNil {
			rw.Header().Set(HeaderCassettePathType, string(pathType))
		}
		if pathName := cassette.PathName(); pathName != "" {
			rw.Header().Set(HeaderCassettePathName, pathName)
		}
		rw.Header().Set(HeaderMode, string(mode))
		rw.Header().Set(HeaderCassetteID, cassette.ID)

		next.ServeHTTP(rw, req)

		res := rw.Result()

		if mode == ModeRecord {
			rw.Header().Set(HeaderSuccess, "true")
			cassette.SetHTTPResponse(req, res)
		} else if mode == ModePlayback {
			rw.Header().Set(HeaderSuccess, fmt.Sprintf("%t", cassette.IsHTTPResponseCorrect(res) && cassette.IsPlaybackSucceeded()))

			rw.Flush()
		}
	})
}

func (p *Playback) NewGRPCMiddleware() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		cassette := p.incomingCassetteFromGRPCContext(ctx)
		mode := cassette.Mode()

		ctx = NewContextWithCassette(ctx, cassette)

		if mode == ModeRecord {
			cassette.SetGRPCRequest(req)
		}

		res, err := handler(ctx, req)

		if mode == ModeRecord {
			cassette.SetGRPCResponse(res)
		}

		md := metadata.Pairs(
			HeaderMode, string(mode),
			HeaderCassetteID, cassette.ID,
		)
		if pathType := cassette.PathType(); pathType != PathTypeNil {
			md.Set(HeaderCassettePathType, string(pathType))
		}
		if pathName := cassette.PathName(); pathName != "" {
			md.Set(HeaderCassettePathName, pathName)
		}
		if mode == ModeRecord {
			md.Set(HeaderSuccess, "true")
		} else if mode == ModePlayback {
			md.Set(HeaderSuccess, fmt.Sprintf("%t", cassette.IsGRPCResponseCorrect(res) && cassette.IsPlaybackSucceeded()))
		}

		grpc.SendHeader(ctx, md)

		return res, err
	}
}

func (p *Playback) incomingCassetteFromHTTPRequest(req *http.Request) *Cassette {
	return p.incomingCassette(req.Context(), req.Header.Get(HeaderCassetteID), req.Header.Get(HeaderMode), req.Header.Get(HeaderCassettePathType), req.Header.Get(HeaderCassettePathName))
}

type MD struct {
	metadata.MD
}

func (meta MD) Get(key string) string {
	values := meta.MD.Get(key)
	if len(values) == 0 {
		return ""
	}

	return values[0]
}

func (p *Playback) incomingCassetteFromGRPCContext(ctx context.Context) *Cassette {
	md, _ := metadata.FromIncomingContext(ctx)
	meta := MD{md}

	return p.incomingCassette(ctx, meta.Get(HeaderCassetteID), meta.Get(HeaderMode), meta.Get(HeaderCassettePathType), meta.Get(HeaderCassettePathName))
}

func (p *Playback) incomingCassette(ctx context.Context, cassetteID, mode, pathType, pathName string) *Cassette {
	cassette := CassetteFromContext(ctx)
	if cassette == nil {
		if cassetteID != "" {
			cassette = p.cassettes[cassetteID]
			cassette.SetMode(ModePlayback).Rewind()
		}
	}
	if cassette == nil && PathType(pathType) == PathTypeFile {
		cassette, _ = p.CassetteFromFile(pathName)
	}
	if cassette == nil {
		cassette, _ = p.NewCassette()
		if Mode(mode) == ModeRecord {
			cassette.SetMode(ModeRecord)

			if PathType(pathType) == PathTypeFile {
				cassette.WithFile()
			}
		}
	}

	return cassette
}
