package playback

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
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

func (r httpResponseRecord) Marshal() string {
	text := fmt.Sprintf("StatusCode: %d\n", r.StatusCode)
	text += fmt.Sprintf("Body:<<BODY\n%s\nBODY\n", r.Body)

	return text
}

var httpResponseRecordRE = regexp.MustCompile(`^\s*(?s)StatusCode:\s+(\d+)\nBody:<<BODY\n(.*)\nBODY[\n\s]*$`)

func (r *httpResponseRecord) Unmarshal(text string) error {
	match := httpResponseRecordRE.FindStringSubmatch(text)

	if len(match) != 3 {
		return fmt.Errorf("Incorrect text on httpResponseRecord.Unmarshal:\n%s\n", text)
	}

	statusCode, err := strconv.Atoi(match[1])
	if err != nil {
		return err
	}

	*r = httpResponseRecord{
		StatusCode: statusCode,
		Body:       match[2],
	}

	return nil
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

	responseRec := &httpResponseRecord{}
	err = responseRec.Unmarshal(rec.response)
	if err != nil {
		return nil, errPlaybackFailed
	}

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

	rec.response, rec.err = responseRec.Marshal(), err

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
