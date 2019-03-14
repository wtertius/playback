package playback

import "net/http"

type httpRecorder struct {
	httpPlayback *httpPlayback
	req          *http.Request
	res          *http.Response
	err          error
}

func newHTTPRecorder(httpPlayback *httpPlayback, req *http.Request) *httpRecorder {
	recorder := &httpRecorder{
		httpPlayback: httpPlayback,
		req:          req,
	}

	return recorder
}

func (r *httpRecorder) Call() error {
	r.res, r.err = r.httpPlayback.Real.RoundTrip(r.req)

	return r.err
}

func (r *httpRecorder) Playback() error {
	r.res, r.err = r.httpPlayback.Playback(r.req)

	return r.err
}

func (r *httpRecorder) Record() error {
	r.res, r.err = r.httpPlayback.Record(r.req)

	return r.err
}
