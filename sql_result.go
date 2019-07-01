package playback

import (
	"context"
	"database/sql/driver"
	"fmt"
)

type sqlResultRecorder struct {
	cassette *Cassette
	rec      *record

	execer        driver.Execer
	execerContext driver.ExecerContext

	ctx         context.Context
	query       string
	namedValues []driver.NamedValue
	values      []driver.Value
	result      driver.Result
	err         error
}

func newSQLResultRecorder(ctx context.Context, query string) *sqlResultRecorder {
	recorder := &sqlResultRecorder{
		cassette: CassetteFromContext(ctx),

		ctx:   ctx,
		query: query,
	}

	return recorder
}

func (r *sqlResultRecorder) WithExecerContext(execerContext driver.ExecerContext) *sqlResultRecorder {
	r.execerContext = execerContext
	return r
}

func (r *sqlResultRecorder) WithExecer(execer driver.Execer) *sqlResultRecorder {
	r.execer = execer
	return r
}

func (r *sqlResultRecorder) WithNamedValues(namedValues []driver.NamedValue) *sqlResultRecorder {
	r.namedValues = namedValues
	return r
}

func (r *sqlResultRecorder) WithValues(values []driver.Value) *sqlResultRecorder {
	r.values = values
	return r
}

func (r *sqlResultRecorder) Call() error {
	r.result, r.err = r.call(r.ctx, r.query)
	return r.err
}

func (r *sqlResultRecorder) call(ctx context.Context, query string) (driver.Result, error) {
	defer func() {
		if r.rec == nil {
			return
		}

		if recovered := recover(); recovered != nil {
			r.rec.Panic = recovered
		}
	}()

	if r.execerContext != nil {
		return r.execerContext.ExecContext(ctx, query, r.namedValues)
	}

	return r.execer.Exec(query, r.values)
}

func (r *sqlResultRecorder) Record() error {
	r.result, r.err = r.record(r.ctx, r.query, r.namedValues)

	return r.err
}

func (r *sqlResultRecorder) record(ctx context.Context, query string, namedValues []driver.NamedValue) (driver.Result, error) {
	rec := r.newRecord(ctx, query)
	if rec == nil {
		return r.call(ctx, query)
	}

	r.rec.RecordRequest()

	result, err := r.call(ctx, query)

	r.RecordResponse(result, err)
	r.rec.PanicIfHas()

	return r.result, err
}

func (r *sqlResultRecorder) RecordResponse(result driver.Result, err error) {
	mockResult := NewMockSQLDriverResultFrom(result)
	r.result = mockResult
	r.rec.Response = string(mockResult.Marshal())

	r.rec.Err = RecordError{err}

	r.rec.Record()
}

func (r *sqlResultRecorder) Playback() error {
	r.result, r.err = r.playback(r.ctx, r.query, r.namedValues)

	return r.err
}

func (r *sqlResultRecorder) playback(ctx context.Context, query string, namedValues []driver.NamedValue) (driver.Result, error) {
	rec := r.newRecord(ctx, query)
	if rec == nil {
		return nil, ErrPlaybackFailed
	}

	err := rec.Playback()
	if err != nil {
		return nil, err
	}

	result := NewMockSQLDriverResult()
	err = result.Unmarshal([]byte(r.rec.Response))
	if err != nil {
		return nil, ErrPlaybackFailed
	}

	rec.PanicIfHas()

	return result, rec.Err.error
}

func (r *sqlResultRecorder) newRecord(ctx context.Context, query string) *record {
	requestDump := fmt.Sprintf("%s\n%#v\n%#v\n", query, r.namedValues, r.values)

	r.rec = &record{
		Kind:     KindSQLResult,
		Key:      query,
		Request:  requestDump,
		cassette: r.cassette,
	}

	return r.rec
}
