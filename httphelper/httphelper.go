package httphelper

import (
	"bytes"
	"io/ioutil"
	"net/http"
)

func ResponseFromBytes(data []byte) *http.Response {
	header := make(http.Header)

	header.Set("Content-Type", "text/plain; charset=utf-8")
	res := &http.Response{
		ProtoMajor: 1,
		ProtoMinor: 1,

		StatusCode: http.StatusOK,
		Header:     header,
		Body:       ioutil.NopCloser(bytes.NewBuffer(data)),
	}

	return res
}

func ResponseFromString(data string) *http.Response {
	return ResponseFromBytes([]byte(data))
}

func ResponseError(statusCode int) *http.Response {
	res := &http.Response{
		ProtoMajor: 1,
		ProtoMinor: 1,

		StatusCode:    statusCode,
		Close:         true,
		ContentLength: -1,
	}

	return res
}
