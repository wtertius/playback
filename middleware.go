package playback

import (
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

		cassette.SetHTTPRequest(req)

		w.Header().Set(HeaderCassettePathType, string(cassette.PathType()))
		w.Header().Set(HeaderCassettePathName, cassette.PathName())
		w.Header().Set(HeaderMode, string(p.Mode))

		// TODO Record request
		// TODO Record response

		next.ServeHTTP(w, req)
	})
}
