package playback

import "io/ioutil"

const BasenamePrefix = "/tmp/playback."

type record struct {
	basename string
	request  string
	response string
	err      error
}

func (r *record) RecordRequest() {
	ioutil.WriteFile(r.requestFilename(), []byte(r.request), 0644)
}

func (r *record) RecordResponse() {
	ioutil.WriteFile(r.responseFilename(), []byte(r.response), 0644)
}

func (r *record) Playback() error {
	err := r.PlaybackRequest()
	if err != nil {
		return err
	}

	return r.PlaybackResponse()
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
	return r.basename + ".request"
}

func (r *record) responseFilename() string {
	return r.basename + ".response"
}
