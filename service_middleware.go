package playback

import (
	"fmt"
	"io/ioutil"
	"net/http"

	yaml "gopkg.in/yaml.v2"
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
	mux.HandleFunc("/playback/get/", handler.ServiceGet)
	mux.HandleFunc("/playback/delete/", handler.ServiceDelete)
	mux.HandleFunc("/playback/list/", handler.ServiceList)

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
	if req.Method != "POST" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

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

func (h *playbackHTTPHandler) ServiceGet(w http.ResponseWriter, req *http.Request) {
	if req.Method != "GET" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	cassetteID := req.URL.Query().Get("id")
	if cassetteID == "" {
		w.WriteHeader(http.StatusBadRequest)
	}

	cassette := h.playback.Get(cassetteID)
	if cassette == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Write(cassette.MarshalToYAML())
}

func (h *playbackHTTPHandler) ServiceDelete(w http.ResponseWriter, req *http.Request) {
	if req.Method != "DELETE" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	cassetteID := req.URL.Query().Get("id")
	if cassetteID == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	deleted := h.playback.Delete(cassetteID)
	if !deleted {
		w.WriteHeader(http.StatusNotFound)
		return
	}
}

func (h *playbackHTTPHandler) ServiceList(w http.ResponseWriter, req *http.Request) {
	if req.Method != "GET" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	cassettes := h.playback.List()
	cassetteIDs := make([]string, 0, len(cassettes))
	for cassetteID := range cassettes {
		cassetteIDs = append(cassetteIDs, cassetteID)
	}

	encoder := yaml.NewEncoder(w)
	encoder.Encode(cassetteIDs)
	encoder.Close()
}
