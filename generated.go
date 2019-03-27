package playback

import (
	"encoding/json"
	"reflect"
)

type generatedRecorder struct {
	key   string
	typ   reflect.Type
	value interface{}
}

func newGeneratedRecorder(key string, value interface{}) *generatedRecorder {
	return &generatedRecorder{
		key:   key,
		typ:   reflect.TypeOf(value),
		value: value,
	}
}

func (r *generatedRecorder) Call() error {
	return nil
}

func (r *generatedRecorder) Record() error {
	rec := r.newRecord()

	rec.RecordRequest()
	rec.RecordResponse()

	return nil
}

func (r *generatedRecorder) Playback() error {
	rec := r.newRecord()

	err := rec.Playback()
	if err != nil {
		return err
	}

	value := reflect.New(r.typ)

	err = json.Unmarshal([]byte(rec.response), value.Interface())
	if err != nil {
		return errPlaybackFailed
	}

	r.value = value.Elem().Interface()

	return nil
}

func (r *generatedRecorder) newRecord() record {
	valueMarshalled := jsonMarshal(r.value)

	return record{
		basename: r.key + "." + r.typ.String(),
		request:  valueMarshalled,
		response: valueMarshalled,
		err:      nil,
	}
}

func jsonMarshal(value interface{}) string {
	bytes, _ := json.MarshalIndent(value, "", "    ")
	return string(bytes)
}
