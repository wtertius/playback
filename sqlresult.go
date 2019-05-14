package playback

import (
	"database/sql/driver"
	"fmt"
)

type sqlResultRecorder struct {
	query  string
	args   []driver.NamedValue
	f      func() (driver.Result, error)
	result driver.Result
	err    error
}

func newSQLResultRecorder(query string, args []driver.NamedValue, f func() (driver.Result, error)) *sqlResultRecorder {
	return &sqlResultRecorder{
		query: query,
		args:  args,
		f:     f,
	}
}

func (r *sqlResultRecorder) Call() error {
	r.result, r.err = r.f()

	return r.err
}

func (r *sqlResultRecorder) Record() error {
	rec := r.newRecord()

	rec.RecordRequest()
	r.Call()
	r.RecordResponse(rec)

	return r.err
}

func (r *sqlResultRecorder) Playback() error {
	rec := r.newRecord()

	err := rec.Playback()
	if err != nil {
		return err
	}

	result := NewMockSQLDriverResult()
	err = result.Unmarshal([]byte(rec.response))
	if err != nil {
		return ErrPlaybackFailed
	}

	r.result, r.err = result, rec.err

	return nil
}

func (r *sqlResultRecorder) newRecord() record {
	args := ""
	for _, a := range r.args {
		args += fmt.Sprintf("%v", a)
	}

	return record{
		basename: "sql_result." + calcMD5(r.query+args),
		request:  r.query,
	}
}

func (r *sqlResultRecorder) RecordResponse(rec record) {
	if r.result == nil {
		rec.RecordResponse()
		return
	}

	result := NewMockSQLDriverResultFrom(r.result)
	r.result = result
	rec.response = string(result.Marshal())

	rec.RecordResponse()
}
