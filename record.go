package playback

import (
	"errors"
	"os"
	"time"

	yaml "gopkg.in/yaml.v2"
)

type RecordKind string

const (
	BasenamePrefix = "playback."

	KindResult = RecordKind("result")
)

var errPlaybackFailed = errors.New("Playback failed")

type record struct {
	// TODO Obsolete - check
	debounce time.Duration
	basename string // TODO REMOVEME
	file     *os.File
	request  string
	response string
	err      error

	// TODO New fields
	Kind     RecordKind
	Key      string
	Request  string
	Response string

	cassette *cassette
}

func (r *record) RecordRequest() {
	//r.Write(r.casseteFile(), r.request)
}

func (r *record) RecordResponse() {
	r.cassette.Add(r)
}

func yamlMarshal(value interface{}) string {
	bytes, _ := yaml.Marshal(value)
	return string(bytes)
}

func (r *record) Playback() error {
	err := r.playback()
	if err != nil {
		return errPlaybackFailed
	}

	return nil
}

func (r *record) playback() error {
	record, err := r.cassette.Get(r.Kind, r.Key)
	if err != nil {
		return err
	}

	r.Request = record.Request
	r.Response = record.Response

	return nil
}
