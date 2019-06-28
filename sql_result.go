package playback

import (
	"context"
	"database/sql/driver"
	"fmt"
)

type sqlResultRecorder struct {
	execerContext driver.ExecerContext
	cassette      *Cassette
	rec           *record

	ctx   context.Context
	query string
	args  []driver.NamedValue
	rows  driver.Result
	err   error
}

func newSQLResultRecorder(ctx context.Context, execerContext driver.ExecerContext, query string, args []driver.NamedValue) *sqlResultRecorder {
	recorder := &sqlResultRecorder{
		execerContext: execerContext,
		cassette:      CassetteFromContext(ctx),

		ctx:   ctx,
		query: query,
		args:  args,
	}

	return recorder
}

func (r *sqlResultRecorder) Call() error {
	r.rows, r.err = r.call(r.ctx, r.query, r.args)
	return r.err
}

func (r *sqlResultRecorder) call(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	defer func() {
		if r.rec == nil {
			return
		}

		if recovered := recover(); recovered != nil {
			r.rec.Panic = recovered
		}
	}()

	return r.execerContext.ExecContext(ctx, query, args)
}

func (r *sqlResultRecorder) Record() error {
	r.rows, r.err = r.record(r.ctx, r.query, r.args)

	return r.err
}

func (r *sqlResultRecorder) record(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	rec := r.newRecord(ctx, query, args)
	if rec == nil {
		return r.call(ctx, query, args)
	}

	r.rec.RecordRequest()

	rows, err := r.call(ctx, query, args)

	r.RecordResponse(rows, err)
	r.rec.PanicIfHas()

	return r.rows, err
}

func (r *sqlResultRecorder) RecordResponse(rows driver.Result, err error) {
	mockResult := NewMockSQLDriverResultFrom(rows)
	r.rows = mockResult
	r.rec.Response = string(mockResult.Marshal())

	r.rec.Err = RecordError{err}

	r.rec.Record()
}

func (r *sqlResultRecorder) Playback() error {
	r.rows, r.err = r.playback(r.ctx, r.query, r.args)

	return r.err
}

func (r *sqlResultRecorder) playback(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	rec := r.newRecord(ctx, query, args)
	if rec == nil {
		return nil, ErrPlaybackFailed
	}

	err := rec.Playback()
	if err != nil {
		return nil, err
	}

	rows := NewMockSQLDriverResult()
	err = rows.Unmarshal([]byte(r.rec.Response))
	if err != nil {
		return nil, ErrPlaybackFailed
	}

	rec.PanicIfHas()

	return rows, rec.Err.error
}

func (r *sqlResultRecorder) newRecord(ctx context.Context, query string, args []driver.NamedValue) *record {
	requestDump := fmt.Sprintf("%s\n%#v\n", query, args)

	r.rec = &record{
		Kind:     KindSQLResult,
		Key:      query,
		Request:  requestDump,
		cassette: r.cassette,
	}

	return r.rec
}
