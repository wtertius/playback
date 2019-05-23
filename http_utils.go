package playback

import (
	"bufio"
	"crypto/md5"
	"fmt"
	"net/http"
	"net/http/httputil"
	"strings"

	"github.com/moul/http2curl"
)

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

func httpDumpResponse(res *http.Response) string {
	if res == nil {
		return ""
	}

	response, _ := httputil.DumpResponse(res, true)
	return string(response)
}

func calcMD5(data []byte) string {
	return fmt.Sprintf("%x", md5.Sum(data))
}

func requestToCurl(req *http.Request) (curl string) {
	command, _ := http2curl.GetCurlCommand(req)
	return command.String()
}
