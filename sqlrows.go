package playback

import (
	"database/sql/driver"
	"fmt"
)

type sqlRowsRecorder struct {
	query string
	args  []driver.NamedValue
	f     func() (driver.Rows, error)
	rows  driver.Rows
	err   error
}

func newSQLRowsRecorder(query string, args []driver.NamedValue, f func() (driver.Rows, error)) *sqlRowsRecorder {
	return &sqlRowsRecorder{
		query: query,
		args:  args,
		f:     f,
	}
}

func (r *sqlRowsRecorder) Call() error {
	r.rows, r.err = r.f()

	return r.err
}

func (r *sqlRowsRecorder) Record() error {
	rec := r.newRecord()

	rec.RecordRequest()
	r.Call()
	r.RecordResponse(rec)

	return r.err
}

func (r *sqlRowsRecorder) Playback() error {
	rec := r.newRecord()

	err := rec.Playback()
	if err != nil {
		return err
	}

	rows := NewMockSQLDriverRows()
	err = rows.Unmarshal([]byte(rec.response))
	if err != nil {
		return ErrPlaybackFailed
	}

	r.rows, r.err = rows, rec.err

	return nil
}

func (r *sqlRowsRecorder) newRecord() record {
	args := ""
	for _, a := range r.args {
		args += fmt.Sprintf("%v", a)
	}

	return record{
		basename: "sql_rows." + calcMD5([]byte(r.query+args)),
		request:  r.query,
	}
}

func (r *sqlRowsRecorder) RecordResponse(rec record) {
	if r.rows == nil {
		rec.RecordResponse()
		return
	}

	rows := NewMockSQLDriverRowsFrom(r.rows)
	r.rows = rows
	rec.response = string(rows.Marshal())

	rec.RecordResponse()
}
