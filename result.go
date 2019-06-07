package playback

import (
	"reflect"

	yaml "gopkg.in/yaml.v2"
)

type resultRecorder struct {
	cassette  *Cassette
	key       string
	typ       reflect.Type
	typString string
	value     interface{}
	panic     interface{}
	err       error
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
	r.applyIfFunc()

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

	rec.ResponseMeta = r.typ.String()
	if r.typString != "" {
		rec.ResponseMeta = r.typString
	}
	rec.Response = yamlMarshalString(r.value)
	rec.Panic = r.panic
	rec.Err = RecordError{r.err}

	rec.Record()

	return rec
}

func (r *resultRecorder) Playback() (err error) {
	defer func() {
		if err != nil {
			r.value = reflect.Zero(r.typ).Interface()
		}
	}()

	rec := r.newRecord()

	err = rec.Playback()
	if err != nil {
		return err
	}

	if rec.Response == "" {
		return ErrPlaybackFailed
	}

	value := reflect.New(r.typ).Interface()
	err = yaml.Unmarshal([]byte(rec.Response), value)
	if err != nil || rec.ResponseMeta != r.typ.String() {
		return ErrPlaybackFailed
	}

	r.value = reflect.ValueOf(value).Elem().Interface()
	r.err = rec.Err.error
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

var errorInterface = reflect.TypeOf((*error)(nil)).Elem()

func (r *resultRecorder) fillInTyp() {
	val := reflect.ValueOf(r.value)
	if val.Kind() != reflect.Func {
		r.typ = val.Type()
		return
	}

	typ := val.Type()
	if typ.NumIn() > 0 || typ.NumOut() < 1 || typ.NumOut() > 2 || (typ.NumOut() == 2 && !typ.Out(1).Implements(errorInterface)) {
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
	if len(results) == 2 && !results[1].IsNil() {
		r.err = results[1].Interface().(error)
	}
}
