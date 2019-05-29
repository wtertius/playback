package playback

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

func (p *Playback) NewHTTPServiceMiddleware(next http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/playback/", p.NewPlaybackHTTPHandler())
	mux.Handle("/", next)

	return mux
}

func (p *Playback) NewPlaybackHTTPHandler() http.Handler {
	handler := playbackHTTPHandler{playback: p}

	mux := http.NewServeMux()
	mux.HandleFunc("/playback/add/", handler.ServiceAdd)

	handler.mux = mux

	return handler
}

type playbackHTTPHandler struct {
	mux      *http.ServeMux
	playback *Playback
}

func (h playbackHTTPHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	handler, _ := h.mux.Handler(req)
	handler.ServeHTTP(w, req)
}

func (h *playbackHTTPHandler) ServiceAdd(w http.ResponseWriter, req *http.Request) {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	cassette, err := h.playback.CassetteFromYAML(body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	fmt.Fprintf(w, cassette.ID)
}
