package playback

import (
	"context"
	"database/sql/driver"
	"encoding/json"

	sqlmwdriver "github.com/wtertius/sqlmw/sql/driver"
)

type MockSQLDriverStmt struct {
	StmtQuery    string
	StmtNumInput int

	ctx  context.Context
	stmt driver.Stmt
}

func NewMockSQLDriverStmtFrom(ctx context.Context, stmtSource driver.Stmt, query string) *MockSQLDriverStmt {
	stmt := &MockSQLDriverStmt{
		StmtQuery:    query,
		StmtNumInput: stmtSource.NumInput(),

		ctx:  ctx,
		stmt: stmtSource,
	}

	return stmt
}

func NewMockSQLDriverStmt(ctx context.Context, query string) *MockSQLDriverStmt {
	return &MockSQLDriverStmt{
		ctx:       ctx,
		StmtQuery: query,
	}
}

func (stmt *MockSQLDriverStmt) Close() error {
	if stmt.stmt != nil {
		return stmt.stmt.Close()
	}

	return nil
}

func (stmt *MockSQLDriverStmt) NumInput() int {
	return stmt.StmtNumInput
}

func (stmt *MockSQLDriverStmt) Exec(args []driver.Value) (driver.Result, error) {
	recorder := newSQLResultRecorder(stmt.ctx, stmt.StmtQuery).
		WithExecer(sqlmwdriver.ExecerFunc(
			func(query string, args []driver.Value) (driver.Result, error) {
				return stmt.stmt.Exec(args)
			},
		)).
		WithValues(args)
	recorder.cassette.Run(recorder)

	return recorder.result, recorder.err
}

func (stmt *MockSQLDriverStmt) Query(args []driver.Value) (driver.Rows, error) {
	recorder := newSQLRowsRecorder(stmt.ctx, stmt.StmtQuery).
		WithQueryer(sqlmwdriver.QueryerFunc(
			func(query string, args []driver.Value) (driver.Rows, error) {
				return stmt.stmt.Query(args)
			},
		)).
		WithValues(args)
	recorder.cassette.Run(recorder)

	return recorder.rows, recorder.err
}

func (stmt *MockSQLDriverStmt) Marshal() []byte {
	dump, _ := json.Marshal(stmt)
	return dump
}

func (stmt *MockSQLDriverStmt) Unmarshal(data []byte) error {
	return json.Unmarshal(data, stmt)
}
