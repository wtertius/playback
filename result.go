package playback

import (
	"os"
	"reflect"

	yaml "gopkg.in/yaml.v2"
)

type resultRecorder struct {
	file  *os.File
	key   string
	typ   reflect.Type
	value interface{}
}

type resultResponse struct {
	Type  string
	Value interface{}
}

func newResultRecorder(file *os.File, key string, value interface{}) *resultRecorder {
	return &resultRecorder{
		file:  file,
		key:   key,
		typ:   reflect.TypeOf(value),
		value: value,
	}
}

func (r *resultRecorder) Call() error {
	return nil
}

func (r *resultRecorder) Record() error {
	rec := r.newRecord()

	rec.RecordResponse()

	return nil
}

func (r *resultRecorder) Playback() error {
	rec := r.newRecord()

	err := rec.Playback()
	if err != nil {
		return err
	}

	if rec.Response == "" {
		return errPlaybackFailed
	}

	var response *resultResponse
	err = yaml.Unmarshal([]byte(rec.Response), &response)
	if err != nil || response.Type != r.typ.String() {
		return errPlaybackFailed
	}

	r.value = response.Value

	return nil
}

func (r *resultRecorder) newRecord() record {
	response := yamlMarshal(&resultResponse{
		Type:  r.typ.String(),
		Value: r.value,
	})

	return record{
		file:     r.file,
		Kind:     KindResult,
		Key:      r.key,
		Response: response,
	}
}
