package playback

import (
	"errors"
	"os"
	"time"

	yaml "gopkg.in/yaml.v2"
)

type RecordKind string

const (
	FileMask = "playback.*.yml"

	KindResult      = RecordKind("result")
	KindHTTP        = RecordKind("http")
	KindHTTPRequest = RecordKind("http_request")
)

var ErrPlaybackFailed = errors.New("Playback failed")

type record struct {
	// TODO Obsolete - check
	debounce time.Duration
	basename string // TODO REMOVEME
	file     *os.File
	request  string
	response string

	// TODO New fields
	Kind        RecordKind
	Key         string
	ID          uint64
	Request     string
	RequestDump string
	Response    string
	Err         RecordError

	cassette *Cassette
}

func (r *record) Record() {
	r.cassette.Add(r)
}

func (r *record) RecordRequest() {
	// FIXME Remove me
	r.Record()
}

func (r *record) RecordResponse() {
	// FIXME Remove me
	r.Record()
}

func (r *record) setID(id uint64) {
	r.ID = id
}

func yamlMarshal(value interface{}) string {
	bytes, _ := yaml.Marshal(value)
	return string(bytes)
}

func (r *record) Playback() error {
	err := r.playback()
	if err != nil {
		return ErrPlaybackFailed
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
	r.Err = record.Err

	return nil
}
