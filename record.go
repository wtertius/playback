package playback

import (
	"errors"
	"io/ioutil"
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
}

func (r *record) RecordRequest() {
	r.Write(r.casseteFile(), r.request)
}

func (r *record) RecordResponse() {
	record := yamlMarshal([]*record{r})
	r.Write(r.casseteFile(), record)
}

func yamlMarshal(value interface{}) string {
	bytes, _ := yaml.Marshal(value)
	return string(bytes)
}

func (r *record) Write(file *os.File, content string) {
	file.WriteString(content)
	file.Sync()
}

func (r *record) Playback() error {
	err := r.playback()
	if err != nil {
		return errPlaybackFailed
	}

	return nil
}

func (r *record) playback() error {
	records, err := r.UnmarshalFromFile()
	if err != nil {
		return err
	}

	for _, record := range records {
		if record.Kind == r.Kind && record.Key == r.Key {
			r.Request = record.Request
			r.Response = record.Response
			break
		}
	}

	return nil
}

func (r *record) UnmarshalFromFile() ([]*record, error) {
	request, err := ioutil.ReadFile(r.casseteFile().Name())
	if err != nil {
		return nil, err
	}

	if len(request) == 0 {
		return nil, errPlaybackFailed
	}

	var records []*record
	err = yaml.Unmarshal(request, &records)
	if err != nil {
		return nil, err
	}

	return records, nil
}

func (r *record) casseteFile() *os.File {
	return r.file
}
