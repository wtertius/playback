package playback

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/moul/http2curl"
)

var _ http.RoundTripper = httpPlayback{}

type httpPlayback struct {
	Real     http.RoundTripper
	playback *Playback
}

type httpResponseRecord struct {
	StatusCode int
	Body       string
}

func (p httpPlayback) RoundTrip(req *http.Request) (res *http.Response, err error) {
	recorder := newHTTPRecorder(&p, req)
	p.playback.Run(recorder)

	return recorder.res, recorder.err
}

func (p *httpPlayback) Playback(req *http.Request) (*http.Response, error) {
	rec := p.newRecord(req)
	err := rec.Playback()
	if err != nil {
		return nil, err
	}

	var responseRec httpResponseRecord
	_ = json.Unmarshal([]byte(rec.response), &responseRec)
	res := http.Response{
		StatusCode: responseRec.StatusCode,
		Body:       ioutil.NopCloser(bytes.NewBuffer([]byte(responseRec.Body))),
	}

	return &res, rec.err
}

func (p *httpPlayback) Record(req *http.Request) (*http.Response, error) {
	rec := p.newRecord(req)

	rec.RecordRequest()

	res, err := p.Real.RoundTrip(req)

	p.RecordResponse(rec, res, err)

	return res, err
}

func (p *httpPlayback) RecordResponse(rec record, res *http.Response, err error) {
	if res == nil {
		rec.RecordResponse()
		return
	}

	responseRec := httpResponseRecord{
		StatusCode: res.StatusCode,
	}

	if res.Body != http.NoBody {
		body, _ := ioutil.ReadAll(res.Body)
		res.Body = ioutil.NopCloser(bytes.NewBuffer(body))

		responseRec.Body = string(body)
	}

	response, _ := json.MarshalIndent(responseRec, "", "    ")

	rec.response, rec.err = string(response), err

	rec.RecordResponse()
}

func (p *httpPlayback) newRecord(req *http.Request) record {
	header := req.Header

	req.Header = p.excludeHeader(req.Header)
	command, _ := http2curl.GetCurlCommand(req)

	req.Header = header

	basename := strings.Replace(req.URL.Path, "/", "", -1) + "_" + calcMD5(command.String())

	return record{
		debounce: p.playback.Debounce,
		basename: basename,
		request:  command.String(),
	}
}

func (p *httpPlayback) excludeHeader(header http.Header) http.Header {
	filtered := make(http.Header, len(header))

	for header, value := range header {
		if p.playback.ExcludeHeaderRE != nil && p.playback.ExcludeHeaderRE.MatchString(header) {
			continue
		}

		filtered[header] = value
	}

	return filtered
}

func calcMD5(data string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(data)))
}
