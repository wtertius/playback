package playback

import (
	"net/http"
	"net/http/httputil"
)

type httpRecorder struct {
	httpPlayback *httpPlayback
	cassette     *Cassette
	req          *http.Request
	res          *http.Response
	err          error
}

func newHTTPRecorder(httpPlayback *httpPlayback, req *http.Request) *httpRecorder {
	recorder := &httpRecorder{
		httpPlayback: httpPlayback,
		cassette:     CassetteFromContext(req.Context()),
		req:          req,
	}

	return recorder
}

func (r *httpRecorder) Call() error {
	r.res, r.err = r.httpPlayback.Real.RoundTrip(r.req)

	return r.err
}

func (r *httpRecorder) Playback() error {
	r.res, r.err = r.playback(r.req)

	return r.err
}

func (r *httpRecorder) playback(req *http.Request) (*http.Response, error) {
	rec := r.newRecord(req)
	if rec == nil {
		return nil, ErrPlaybackFailed
	}

	err := rec.Playback()
	if err != nil {
		return nil, err
	}

	res, err := httpReadResponse(rec.Response, req)
	if err != nil {
		return nil, ErrPlaybackFailed
	}

	rec.PanicIfHas()

	return res, rec.Err.error
}

func (r *httpRecorder) Record() error {
	r.res, r.err = r.record(r.req)

	return r.err
}

func (r *httpRecorder) record(req *http.Request) (*http.Response, error) {
	rec := r.newRecord(req)
	if rec == nil {
		return r.call(rec, req)
	}

	rec.RecordRequest()

	res, err := r.call(rec, req)

	r.RecordResponse(rec, res, err)
	rec.PanicIfHas()

	return res, err
}

func (r *httpRecorder) call(rec *record, req *http.Request) (*http.Response, error) {
	defer func() {
		if rec == nil {
			return
		}

		if recovered := recover(); recovered != nil {
			rec.Panic = recovered
		}
	}()

	return r.httpPlayback.Real.RoundTrip(req)
}

func (r *httpRecorder) RecordResponse(rec *record, res *http.Response, err error) {
	rec.Response = httpDumpResponse(res)
	rec.Err = RecordError{err}

	rec.Record()
}

func (r *httpRecorder) newRecord(req *http.Request) *record {
	if r.cassette == nil {
		return nil
	}

	header := req.Header

	curl := requestToCurl(req)
	requestDump, _ := httputil.DumpRequest(req, true)
	key := req.URL.Path + "?" + calcMD5(requestDump)

	req.Header = header

	return &record{
		Kind:        KindHTTP,
		Key:         key,
		Request:     curl,
		RequestDump: string(requestDump),
		cassette:    r.cassette,
	}
}
