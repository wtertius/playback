package playback

import (
	"net/http"
	"net/http/httputil"
)

type HTTPRecorder struct {
	httpPlayback *httpPlayback
	cassette     *Cassette
	rec          *record
	req          *http.Request
	res          *http.Response
	err          error
}

func newHTTPRecorder(httpPlayback *httpPlayback, req *http.Request) *HTTPRecorder {
	recorder := &HTTPRecorder{
		httpPlayback: httpPlayback,
		cassette:     CassetteFromContext(req.Context()),
		req:          req,
	}

	return recorder
}

func (r *HTTPRecorder) Call() error {
	r.res, r.err = r.httpPlayback.Real.RoundTrip(r.req)

	return r.err
}

func (r *HTTPRecorder) Playback() error {
	r.res, r.err = r.playback(r.req)

	return r.err
}

func (r *HTTPRecorder) playback(req *http.Request) (*http.Response, error) {
	rec := r.newRecord(req)
	if rec == nil {
		return nil, ErrPlaybackFailed
	}

	err := rec.Playback()
	if err != nil {
		r.cassette.debugRecordMatch(rec, KindHTTP, req.URL.Path+"?")

		return nil, err
	}

	res, err := httpReadResponse(rec.Response, req)
	if err != nil {
		return nil, ErrPlaybackFailed
	}

	rec.PanicIfHas()

	return res, rec.Err.error
}

func (r *HTTPRecorder) Record() error {
	r.res, r.err = r.record(r.req)

	return r.err
}

func (r *HTTPRecorder) record(req *http.Request) (*http.Response, error) {
	rec := r.newRecord(req)
	if rec == nil {
		return r.call(req)
	}

	r.rec.RecordRequest()

	res, err := r.call(req)

	r.RecordResponse(res, err)
	r.rec.PanicIfHas()

	return res, err
}

func (r *HTTPRecorder) call(req *http.Request) (*http.Response, error) {
	defer func() {
		if r.rec == nil {
			return
		}

		if recovered := recover(); recovered != nil {
			r.rec.Panic = recovered
		}
	}()

	return r.httpPlayback.Real.RoundTrip(req)
}

func (r *HTTPRecorder) RecordResponse(res *http.Response, err error) {
	r.rec.Response = httpDumpResponse(res)
	r.rec.Err = RecordError{err}

	r.rec.Record()
}

func (r *HTTPRecorder) newRecord(req *http.Request) *record {
	header := req.Header

	curl := requestToCurl(req)
	requestDump, _ := httputil.DumpRequestOut(req, true)
	key := req.URL.Path + "?" + calcMD5(requestDump)

	req.Header = header

	r.rec = &record{
		Kind:        KindHTTP,
		Key:         key,
		RequestMeta: curl,
		Request:     string(requestDump),
		cassette:    r.cassette,
	}

	return r.rec
}
