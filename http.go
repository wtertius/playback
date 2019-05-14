package playback

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"

	"github.com/moul/http2curl"

	yaml "gopkg.in/yaml.v2"
)

var _ http.RoundTripper = httpPlayback{}

type httpPlayback struct {
	Real     http.RoundTripper
	playback *Playback
}

type httpResponseRecord struct {
	StatusCode int
	Header     http.Header
	Body       string
}

func (r httpResponseRecord) Marshal() string {
	text := fmt.Sprintf("StatusCode: %d\n", r.StatusCode)
	text += fmt.Sprintf("Body:<<BODY\n%s\nBODY\n", r.Body)

	return text
}

var httpResponseRecordRE = regexp.MustCompile(`^\s*(?s)StatusCode:\s+(\d+)\nBody:<<BODY\n(.*)\nBODY[\n\s]*$`)

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

	var response *httpResponseRecord
	err = yaml.Unmarshal([]byte(rec.Response), &response)
	if err != nil {
		return nil, ErrPlaybackFailed
	}

	res := http.Response{
		StatusCode: response.StatusCode,
		Body:       ioutil.NopCloser(bytes.NewBuffer([]byte(response.Body))),
		Header:     response.Header,
	}

	return &res, rec.err
}

func (p *httpPlayback) Record(req *http.Request) (*http.Response, error) {
	rec := p.newRecord(req)

	rec.Record()

	res, err := p.Real.RoundTrip(req)

	p.RecordResponse(rec, res, err)

	return res, err
}

func (p *httpPlayback) RecordResponse(rec record, res *http.Response, err error) {
	if res == nil {
		rec.RecordResponse()
		return
	}

	response := httpResponseRecord{
		Header:     res.Header,
		StatusCode: res.StatusCode,
	}

	if res.Body != http.NoBody {
		body, _ := ioutil.ReadAll(res.Body)
		res.Body = ioutil.NopCloser(bytes.NewBuffer(body))

		response.Body = string(body)
	}

	rec.Response = yamlMarshal(response)
	rec.err = err

	rec.Record()
}

func (p *httpPlayback) newRecord(req *http.Request) record {
	key, command := p.requestToCurl(req)

	return record{
		Kind:     KindHTTP,
		Key:      key,
		Request:  command,
		cassette: p.playback.cassette,
	}
}

func (p *httpPlayback) excludeHeader(header http.Header) http.Header {
	return header // FIXME Remove or finish

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

func (p *httpPlayback) requestToCurl(req *http.Request) (key string, curl string) {
	header := req.Header
	req.Header = p.excludeHeader(req.Header)

	key, curl = RequestToCurl(req)

	req.Header = header

	return key, curl
}

func RequestToCurl(req *http.Request) (key string, curl string) {
	command, _ := http2curl.GetCurlCommand(req)
	curl = command.String()

	key = strings.Replace(req.URL.Path, "/", "", -1) + "_" + calcMD5(curl)

	return key, curl
}
