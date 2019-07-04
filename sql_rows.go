package playback

import (
	"context"
	"database/sql/driver"
	"fmt"
)

type SQLRowsRecorder struct {
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

func newSQLRowsRecorder(ctx context.Context, query string) *SQLRowsRecorder {
	recorder := &SQLRowsRecorder{
		cassette: CassetteFromContext(ctx),

		ctx:   ctx,
		query: query,
	}

	return recorder
}

func (r *SQLRowsRecorder) WithQueryerContext(queryerContext driver.QueryerContext) *SQLRowsRecorder {
	r.queryerContext = queryerContext
	return r
}

func (r *SQLRowsRecorder) WithQueryer(queryer driver.Queryer) *SQLRowsRecorder {
	r.queryer = queryer
	return r
}

func (r *SQLRowsRecorder) WithNamedValues(namedValues []driver.NamedValue) *SQLRowsRecorder {
	r.namedValues = namedValues
	return r
}

func (r *SQLRowsRecorder) WithValues(values []driver.Value) *SQLRowsRecorder {
	r.values = values
	return r
}

func (r *SQLRowsRecorder) Call() error {
	r.rows, r.err = r.call(r.ctx, r.query)
	return r.err
}

func (r *SQLRowsRecorder) call(ctx context.Context, query string) (driver.Rows, error) {
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

func (r *SQLRowsRecorder) Record() error {
	r.rows, r.err = r.record(r.ctx, r.query)

	return r.err
}

func (r *SQLRowsRecorder) record(ctx context.Context, query string) (driver.Rows, error) {
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

func (r *SQLRowsRecorder) RecordResponse(rows driver.Rows, err error) {
	mockRows := NewMockSQLDriverRowsFrom(rows)
	r.rows = mockRows
	r.rec.Response = string(mockRows.Marshal())

	r.rec.Err = RecordError{err}

	r.rec.Record()
}

func (r *SQLRowsRecorder) Playback() error {
	r.rows, r.err = r.playback(r.ctx, r.query, r.namedValues)

	return r.err
}

func (r *SQLRowsRecorder) playback(ctx context.Context, query string, namedValues []driver.NamedValue) (driver.Rows, error) {
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

func (r *SQLRowsRecorder) newRecord(ctx context.Context, query string) *record {
	requestDump := fmt.Sprintf("%s\n%#v\n%#v\n", query, r.namedValues, r.values)

	r.rec = &record{
		Kind:     KindSQLRows,
		Key:      query,
		Request:  requestDump,
		cassette: r.cassette,
	}

	return r.rec
}

func (r *SQLRowsRecorder) ApplyOptions(options ...SQLRowsRecorderOption) {
	for _, option := range options {
		r = option(r)
	}
}

type SQLRowsRecorderOption func(r *SQLRowsRecorder) *SQLRowsRecorder

func WithNamedValues(namedValues []driver.NamedValue) SQLRowsRecorderOption {
	return func(r *SQLRowsRecorder) *SQLRowsRecorder {
		return r.WithNamedValues(namedValues)
	}
}

func WithValues(values []driver.Value) SQLRowsRecorderOption {
	return func(r *SQLRowsRecorder) *SQLRowsRecorder {
		return r.WithValues(values)
	}
}
