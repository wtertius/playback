package playback

import (
	"context"
	"database/sql/driver"

	sqlmwdriver "github.com/wtertius/sqlmw/sql/driver"
	"github.com/wtertius/sqlmw/sql/driver/wrapper"
)

func (p *Playback) SQLNameAndDSN(driverName, dsn string) (string, string) {
	chain := wrapper.NewChain(driverName, dsn)
	chain.Add(p.sqlWrapper())

	return chain.NameAndDSN()
}

func (p *Playback) sqlWrapper() sqlmwdriver.Wrapper {
	return &SQLWrapper{}
}

type SQLWrapper struct {
	*sqlmwdriver.CustomWrapper
}

func (w *SQLWrapper) QueryerContext(queryerContext driver.QueryerContext) driver.QueryerContext {
	return sqlmwdriver.QueryerContextFunc(func(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
		recorder := newSQLRowsRecorder(ctx, queryerContext, query, args)
		recorder.cassette.Run(recorder)

		return recorder.rows, recorder.err
	})
}

func (w *SQLWrapper) ExecerContext(execerContext driver.ExecerContext) driver.ExecerContext {
	return sqlmwdriver.ExecerContextFunc(func(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
		recorder := newSQLResultRecorder(ctx, execerContext, query, args)
		recorder.cassette.Run(recorder)

		return recorder.rows, recorder.err
	})
}
