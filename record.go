package playback

import (
	"errors"
	"io/ioutil"
	"time"
)

const BasenamePrefix = "/tmp/playback."

var errPlaybackFailed = errors.New("Playback failed")

type record struct {
	debounce time.Duration
	basename string
	request  string
	response string
	err      error
}

func (r *record) RecordRequest() {
	r.WriteDebounced(r.requestFilename(), r.request)
}

func (r *record) RecordResponse() {
	r.WriteDebounced(r.responseFilename(), r.response)
}

func (r *record) WriteDebounced(filename string, content string) {
	Debounce(filename, func() {
		ioutil.WriteFile(filename, []byte(content), 0644)
	}, r.debounce)
}

func (r *record) Playback() error {
	err := r.PlaybackRequest()
	if err != nil {
		return errPlaybackFailed
	}

	err = r.PlaybackResponse()
	if err != nil {
		return errPlaybackFailed
	}

	return nil
}

func (r *record) PlaybackRequest() error {
	request, err := ioutil.ReadFile(r.requestFilename())
	r.request = string(request)
	return err
}

func (r *record) PlaybackResponse() error {
	response, err := ioutil.ReadFile(r.responseFilename())
	r.response = string(response)
	return err
}

func (r *record) requestFilename() string {
	return BasenamePrefix + r.basename + ".request"
}

func (r *record) responseFilename() string {
	return BasenamePrefix + r.basename + ".response"
}
