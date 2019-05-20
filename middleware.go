package playback

import (
	"fmt"
	"net/http"
)

const (
	HeaderCassettePathType = "x-playback-path-type"
	HeaderCassettePathName = "x-playback-path-name"
	HeaderMode             = "x-playback-mode"
	HeaderSuccess          = "x-playback-success"
)

func (p *Playback) NewMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		cassette := CassetteFromContext(ctx)
		if cassette == nil && PathTypeFile == PathType(req.Header.Get(HeaderCassettePathType)) {
			cassette, _ = p.CassetteFromFile(req.Header.Get(HeaderCassettePathName))
		}
		if cassette == nil {
			cassette, _ = p.NewCassette()
		}

		ctx = NewContextWithCassette(ctx, cassette)
		req = req.WithContext(ctx)

		if p.Mode == ModeRecord {
			cassette.SetHTTPRequest(req)
		}

		rw := multiplexHTTPResponseWriter(w, p.Mode)
		rw.Header().Set(HeaderCassettePathType, string(cassette.PathType()))
		rw.Header().Set(HeaderCassettePathName, cassette.PathName())
		rw.Header().Set(HeaderMode, string(p.Mode))

		next.ServeHTTP(rw, req)

		res := rw.Result()

		if p.Mode == ModeRecord {
			rw.Header().Set(HeaderSuccess, "true")
			cassette.SetHTTPResponse(req, res)
		} else if p.Mode == ModePlayback {
			rw.Header().Set(HeaderSuccess, fmt.Sprintf("%t", cassette.IsHTTPResponseCorrect(res) && cassette.IsPlaybackSucceeded()))

			rw.Flush()
		}
	})
}
