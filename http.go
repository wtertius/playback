package playback

import (
	"bufio"
	"crypto/md5"
	"fmt"
	"net/http"
	"net/http/httputil"
	"regexp"
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

	return res, rec.Err.error
}

func (p *httpPlayback) Record(req *http.Request) (*http.Response, error) {
	rec := p.newRecord(req)
	if rec == nil {
		return p.call(rec, req)
	}

	rec.RecordRequest()

	res, err := p.call(rec, req)

	p.RecordResponse(rec, res, err)
	rec.PanicIfHas()

	return res, err
}

func (p *httpPlayback) call(rec *record, req *http.Request) (*http.Response, error) {
	defer func() {
		if rec == nil {
			return
		}

		if recovered := recover(); recovered != nil {
			rec.Panic = recovered
		}
	}()

	return p.Real.RoundTrip(req)
}

func (p *httpPlayback) RecordResponse(rec *record, res *http.Response, err error) {
	rec.Response = httpDumpResponse(res)
	rec.Err = RecordError{err}

	rec.Record()
}

func httpDumpResponse(res *http.Response) string {
	if res == nil {
		return ""
	}

	response, _ := httputil.DumpResponse(res, true)
	return string(response)
}

func (p *httpPlayback) newRecord(req *http.Request) *record {
	cassette := CassetteFromContext(req.Context())
	if cassette == nil {
		return nil
	}

	header := req.Header
	req.Header = p.excludeHeader(req.Header)

	curl := requestToCurl(req)
	requestDump, _ := httputil.DumpRequest(req, true)
	key := req.URL.Path + "?" + calcMD5(requestDump)

	req.Header = header

	return &record{
		Kind:        KindHTTP,
		Key:         key,
		Request:     curl,
		RequestDump: string(requestDump),
		cassette:    cassette,
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

func calcMD5(data []byte) string {
	return fmt.Sprintf("%x", md5.Sum(data))
}

func requestToCurl(req *http.Request) (curl string) {
	command, _ := http2curl.GetCurlCommand(req)
	return command.String()
}

func httpReadRequest(dump string) (*http.Request, error) {
	return http.ReadRequest(bufioReader(dump))
}

func httpReadResponse(dump string, req *http.Request) (*http.Response, error) {
	if dump == "" {
		return nil, nil
	}
	return http.ReadResponse(bufioReader(dump), req)
}

func bufioReader(str string) *bufio.Reader {
	return bufio.NewReader(strings.NewReader(str))
}

func httpCopyResponse(res *http.Response, req *http.Request) *http.Response {
	res, _ = httpReadResponse(httpDumpResponse(res), req)
	return res
}

func httpDeleteHeaders(res *http.Response) *http.Response {
	res.Header.Del(HeaderMode)
	res.Header.Del(HeaderSuccess)

	return res
}
