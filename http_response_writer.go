package playback

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
)

func multiplexHTTPResponseWriter(w http.ResponseWriter, mode Mode) httpResponseWriter {
	rw := httpResponseWriter{
		master: w,
		slave:  httptest.NewRecorder(),

		isDelayed: mode == ModePlayback,
	}

	rw.slave.HeaderMap = w.Header()

	return rw
}

type httpResponseWriter struct {
	master http.ResponseWriter
	slave  *httptest.ResponseRecorder

	isDelayed bool
}

func (w httpResponseWriter) Header() http.Header {
	return w.master.Header()
}

func (w httpResponseWriter) Write(data []byte) (int, error) {
	n, err := w.slave.Write(data)
	if w.isDelayed {
		return n, err
	}

	return w.master.Write(data)
}

func (w httpResponseWriter) WriteHeader(code int) {
	w.slave.WriteHeader(code)
	if !w.isDelayed {
		w.master.WriteHeader(code)
	}
}

func (w httpResponseWriter) Result() *http.Response {
	return w.slave.Result()
}

func (w httpResponseWriter) Flush() {
	if !w.isDelayed {
		return
	}

	res := w.Result()

	w.master.WriteHeader(res.StatusCode)

	body, _ := ioutil.ReadAll(res.Body)
	w.master.Write(body)
}
