package playback

import (
	"context"
	"database/sql/driver"
	"fmt"
)

type sqlRowsRecorder struct {
	cassette *Cassette
	rec      *record

	queryerContext driver.QueryerContext
	queryer        driver.Queryer

	ctx         context.Context
	query       string
	values      []driver.Value
	namedValues []driver.NamedValue
	rows        driver.Rows
	err         error
}

func newSQLRowsRecorder(ctx context.Context, query string) *sqlRowsRecorder {
	recorder := &sqlRowsRecorder{
		cassette: CassetteFromContext(ctx),

		ctx:   ctx,
		query: query,
	}

	return recorder
}

func (r *sqlRowsRecorder) WithQueryerContext(queryerContext driver.QueryerContext) *sqlRowsRecorder {
	r.queryerContext = queryerContext
	return r
}

func (r *sqlRowsRecorder) WithQueryer(queryer driver.Queryer) *sqlRowsRecorder {
	r.queryer = queryer
	return r
}

func (r *sqlRowsRecorder) WithNamedValues(namedValues []driver.NamedValue) *sqlRowsRecorder {
	r.namedValues = namedValues
	return r
}

func (r *sqlRowsRecorder) WithValues(values []driver.Value) *sqlRowsRecorder {
	r.values = values
	return r
}

func (r *sqlRowsRecorder) Call() error {
	r.rows, r.err = r.call(r.ctx, r.query)
	return r.err
}

func (r *sqlRowsRecorder) call(ctx context.Context, query string) (driver.Rows, error) {
	defer func() {
		if r.rec == nil {
			return
		}

		if recovered := recover(); recovered != nil {
			r.rec.Panic = recovered
		}
	}()

	if r.queryerContext != nil {
		return r.queryerContext.QueryContext(ctx, query, r.namedValues)
	}

	return r.queryer.Query(query, r.values)
}

func (r *sqlRowsRecorder) Record() error {
	r.rows, r.err = r.record(r.ctx, r.query)

	return r.err
}

func (r *sqlRowsRecorder) record(ctx context.Context, query string) (driver.Rows, error) {
	rec := r.newRecord(ctx, query)
	if rec == nil {
		return r.call(ctx, query)
	}

	r.rec.RecordRequest()

	rows, err := r.call(ctx, query)

	r.RecordResponse(rows, err)
	r.rec.PanicIfHas()

	return r.rows, err
}

func (r *sqlRowsRecorder) RecordResponse(rows driver.Rows, err error) {
	mockRows := NewMockSQLDriverRowsFrom(rows)
	r.rows = mockRows
	r.rec.Response = string(mockRows.Marshal())

	r.rec.Err = RecordError{err}

	r.rec.Record()
}

func (r *sqlRowsRecorder) Playback() error {
	r.rows, r.err = r.playback(r.ctx, r.query, r.namedValues)

	return r.err
}

func (r *sqlRowsRecorder) playback(ctx context.Context, query string, namedValues []driver.NamedValue) (driver.Rows, error) {
	rec := r.newRecord(ctx, query)
	if rec == nil {
		return nil, ErrPlaybackFailed
	}

	err := rec.Playback()
	if err != nil {
		return nil, err
	}

	rows := NewMockSQLDriverRows()
	err = rows.Unmarshal([]byte(r.rec.Response))
	if err != nil {
		return nil, ErrPlaybackFailed
	}

	rec.PanicIfHas()

	return rows, rec.Err.error
}

func (r *sqlRowsRecorder) newRecord(ctx context.Context, query string) *record {
	requestDump := fmt.Sprintf("%s\n%#v\n%#v\n", query, r.namedValues, r.values)

	r.rec = &record{
		Kind:     KindSQLRows,
		Key:      query,
		Request:  requestDump,
		cassette: r.cassette,
	}

	return r.rec
}
