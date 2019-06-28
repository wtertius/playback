package playback

import (
	"context"
	"database/sql/driver"
	"fmt"
)

type sqlRowsRecorder struct { // TODO rename to sqlRowsRecorder
	queryerContext driver.QueryerContext
	cassette       *Cassette
	rec            *record

	ctx   context.Context
	query string
	args  []driver.NamedValue
	rows  driver.Rows
	err   error
}

func newSQLRowsRecorder(ctx context.Context, queryerContext driver.QueryerContext, query string, args []driver.NamedValue) *sqlRowsRecorder {
	recorder := &sqlRowsRecorder{
		queryerContext: queryerContext,
		cassette:       CassetteFromContext(ctx),

		ctx:   ctx,
		query: query,
		args:  args,
	}

	return recorder
}

func (r *sqlRowsRecorder) Call() error {
	r.rows, r.err = r.call(r.ctx, r.query, r.args)
	return r.err
}

func (r *sqlRowsRecorder) call(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	defer func() {
		if r.rec == nil {
			return
		}

		if recovered := recover(); recovered != nil {
			r.rec.Panic = recovered
		}
	}()

	return r.queryerContext.QueryContext(ctx, query, args)
}

func (r *sqlRowsRecorder) Record() error {
	r.rows, r.err = r.record(r.ctx, r.query, r.args)

	return r.err
}

func (r *sqlRowsRecorder) record(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
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

func (r *sqlRowsRecorder) RecordResponse(rows driver.Rows, err error) {
	mockRows := newMockSQLDriverRowsFrom(rows)
	r.rows = mockRows
	r.rec.Response = string(mockRows.Marshal())

	r.rec.Err = RecordError{err}

	r.rec.Record()
}

func (r *sqlRowsRecorder) Playback() error {
	r.rows, r.err = r.playback(r.ctx, r.query, r.args)

	return r.err
}

func (r *sqlRowsRecorder) playback(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	rec := r.newRecord(ctx, query, args)
	if rec == nil {
		return nil, ErrPlaybackFailed
	}

	err := rec.Playback()
	if err != nil {
		return nil, err
	}

	rows := newMockSQLDriverRows()
	err = rows.Unmarshal([]byte(r.rec.Response))
	if err != nil {
		return nil, ErrPlaybackFailed
	}

	rec.PanicIfHas()

	return rows, rec.Err.error
}

func (r *sqlRowsRecorder) newRecord(ctx context.Context, query string, args []driver.NamedValue) *record {
	requestDump := fmt.Sprintf("%s\n%#v\n", query, args)

	r.rec = &record{
		Kind:     KindSQLRows,
		Key:      query,
		Request:  requestDump,
		cassette: r.cassette,
	}

	return r.rec
}
