package playback

import (
	"reflect"

	yaml "gopkg.in/yaml.v2"
)

type resultRecorder struct {
	cassette *Cassette
	key      string
	typ      reflect.Type
	value    interface{}
	panic    interface{}
}

type resultResponse struct {
	Type  string
	Value interface{}
}

func newResultRecorder(cassette *Cassette, key string, value interface{}, panicObject interface{}) *resultRecorder {
	r := &resultRecorder{
		cassette: cassette,
		key:      key,
		value:    value,
	}

	r.fillInTyp()

	return r
}

func (r *resultRecorder) Call() error {
	return nil
}

func (r *resultRecorder) Record() error {
	rec := r.record()

	rec.PanicIfHas()

	return nil
}

func (r *resultRecorder) record() record {
	r.applyIfFunc()
	rec := r.newRecord()

	rec.Response = yamlMarshalString(&resultResponse{
		Type:  r.typ.String(),
		Value: r.value,
	})
	rec.Panic = r.panic

	rec.Record()

	return rec
}

func (r *resultRecorder) Playback() error {
	rec := r.newRecord()

	err := rec.Playback()
	if err != nil {
		r.value = reflect.Zero(r.typ).Interface()

		return err
	}

	if rec.Response == "" {
		return ErrPlaybackFailed
	}

	var response *resultResponse
	err = yaml.Unmarshal([]byte(rec.Response), &response)
	if err != nil || response.Type != r.typ.String() {
		return ErrPlaybackFailed
	}

	r.value = response.Value
	rec.PanicIfHas()

	return nil
}

func (r *resultRecorder) newRecord() record {
	return record{
		Kind:     KindResult,
		Key:      r.key,
		cassette: r.cassette,
	}
}

func (r *resultRecorder) fillInTyp() {
	val := reflect.ValueOf(r.value)
	if val.Kind() != reflect.Func {
		r.typ = val.Type()
		return
	}

	typ := val.Type()
	if typ.NumIn() > 0 || typ.NumOut() != 1 {
		// TODO return error
		panic("Incorrect type: " + typ.String())
		return
	}

	r.typ = typ.Out(0)
}

func (r *resultRecorder) applyIfFunc() {
	val := reflect.ValueOf(r.value)
	if val.Kind() != reflect.Func {
		return
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			r.panic = recovered
			r.value = reflect.Zero(val.Type().Out(0)).Interface()
		}
	}()
	results := val.Call([]reflect.Value{})
	r.value = results[0].Interface()
}
