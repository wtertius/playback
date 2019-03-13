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

func (p httpPlayback) RoundTrip(req *http.Request) (*http.Response, error) {
	if !p.playback.On {
		return p.Real.RoundTrip(req)
	}

	if p.playback.Mode == ModePlayback {
		return p.Playback(req)
	}

	return p.Record(req)

	/*
		command, err := http2curl.GetCurlCommand(req)
		if err != nil {
			fmt.Printf("Can't convert request to curl. Error: %s", err)
		}

		filename := strings.Replace(req.URL.Path, "/", "", -1) + "_" + calcMD5(command.String()) + ".playback"
	*/

	// ioutil.WriteFile("/app/go/src/gitlab.ozon.ru/travel/flight-gds-s7/RQ.xml", buf.Bytes(), 0644)

}

func (p *httpPlayback) Playback(req *http.Request) (*http.Response, error) {
	rec := p.newRecordFromHTTPRequest(req)
	rec.Playback()

	var responseRec httpResponseRecord
	_ = json.Unmarshal([]byte(rec.response), &responseRec)
	res := http.Response{
		StatusCode: responseRec.StatusCode,
		Body:       ioutil.NopCloser(bytes.NewBuffer([]byte(responseRec.Body))),
	}

	// TODO get from file an
	return &res, nil
}

func (p *httpPlayback) Record(req *http.Request) (*http.Response, error) {
	rec := p.newRecordFromHTTPRequest(req)

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

type httpResponseRecord struct {
	StatusCode int
	Body       string
}

func (p *httpPlayback) newRecordFromHTTPRequest(req *http.Request) record {
	command, _ := http2curl.GetCurlCommand(req)
	basename := BasenamePrefix + strings.Replace(req.URL.Path, "/", "", -1) + "_" + calcMD5(command.String())

	return record{
		basename: basename,
		request:  command.String(),
	}
}

func calcMD5(data string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(data)))
}
