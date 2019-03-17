package playback

import (
	"encoding/json"
	"reflect"
)

type randomRecorder struct {
	key   string
	typ   reflect.Type
	value interface{}
}

func newRandomRecorder(key string, value interface{}) *randomRecorder {
	return &randomRecorder{
		key:   key,
		typ:   reflect.TypeOf(value),
		value: value,
	}
}

func (r *randomRecorder) Call() error {
	return nil
}

func (r *randomRecorder) Record() error {
	rec := r.newRecord()

	rec.RecordRequest()
	rec.RecordResponse()

	return nil
}

func (r *randomRecorder) Playback() error {
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

func (r *randomRecorder) newRecord() record {
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
