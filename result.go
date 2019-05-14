package playback

import (
	"reflect"

	yaml "gopkg.in/yaml.v2"
)

type resultRecorder struct {
	cassette *cassette
	key      string
	typ      reflect.Type
	value    interface{}
}

type resultResponse struct {
	Type  string
	Value interface{}
}

func newResultRecorder(cassette *cassette, key string, value interface{}) *resultRecorder {
	return &resultRecorder{
		cassette: cassette,
		key:      key,
		typ:      reflect.TypeOf(value),
		value:    value,
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
		r.value = reflect.Zero(r.typ).Interface()

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
		Kind:     KindResult,
		Key:      r.key,
		Response: response,
		cassette: r.cassette,
	}
}
