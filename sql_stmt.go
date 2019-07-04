package playback

import (
	"context"
	"database/sql/driver"
)

type SQLStmtRecorder struct {
	connPrepareContext driver.ConnPrepareContext
	cassette           *Cassette
	rec                *record

	ctx   context.Context
	query string
	stmt  driver.Stmt
	err   error
}

func newSQLStmtRecorder(ctx context.Context, connPrepareContext driver.ConnPrepareContext, query string) *SQLStmtRecorder {
	recorder := &SQLStmtRecorder{
		connPrepareContext: connPrepareContext,
		cassette:           CassetteFromContext(ctx),

		ctx:   ctx,
		query: query,
	}

	return recorder
}

func (r *SQLStmtRecorder) Call() error {
	r.stmt, r.err = r.call(r.ctx, r.query)
	return r.err
}

func (r *SQLStmtRecorder) call(ctx context.Context, query string) (driver.Stmt, error) {
	defer func() {
		if r.rec == nil {
			return
		}

		if recovered := recover(); recovered != nil {
			r.rec.Panic = recovered
		}
	}()

	return r.connPrepareContext.PrepareContext(ctx, query)
}

func (r *SQLStmtRecorder) Record() error {
	r.stmt, r.err = r.record(r.ctx, r.query)

	return r.err
}

func (r *SQLStmtRecorder) record(ctx context.Context, query string) (driver.Stmt, error) {
	rec := r.newRecord(ctx, query)
	if rec == nil {
		return r.call(ctx, query)
	}

	r.rec.RecordRequest()

	stmt, err := r.call(ctx, query)

	r.RecordResponse(ctx, stmt, err)
	r.rec.PanicIfHas()

	return r.stmt, err
}

func (r *SQLStmtRecorder) RecordResponse(ctx context.Context, stmt driver.Stmt, err error) {
	mockStmt := NewMockSQLDriverStmtFrom(ctx, stmt, r.query)
	r.stmt = mockStmt
	r.rec.Response = string(mockStmt.Marshal())

	r.rec.Err = RecordError{err}

	r.rec.Record()
}

func (r *SQLStmtRecorder) Playback() error {
	r.stmt, r.err = r.playback(r.ctx, r.query)

	return r.err
}

func (r *SQLStmtRecorder) playback(ctx context.Context, query string) (driver.Stmt, error) {
	rec := r.newRecord(ctx, query)
	if rec == nil {
		return nil, ErrPlaybackFailed
	}

	err := rec.Playback()
	if err != nil {
		return nil, err
	}

	stmt := NewMockSQLDriverStmt(ctx, query)
	err = stmt.Unmarshal([]byte(r.rec.Response))
	if err != nil {
		return nil, ErrPlaybackFailed
	}

	rec.PanicIfHas()

	return stmt, rec.Err.error
}

func (r *SQLStmtRecorder) newRecord(ctx context.Context, query string) *record {
	r.rec = &record{
		Kind:     KindSQLStmt,
		Key:      query,
		Request:  query,
		cassette: r.cassette,
	}

	return r.rec
}
