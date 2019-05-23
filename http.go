package playback

import (
	"net/http"
)

var _ http.RoundTripper = httpPlayback{}

type httpPlayback struct {
	Real     http.RoundTripper
	playback *Playback
}

func (p httpPlayback) RoundTrip(req *http.Request) (res *http.Response, err error) {
	recorder := newHTTPRecorder(&p, req)
	recorder.cassette.Run(recorder)

	return recorder.res, recorder.err
}
