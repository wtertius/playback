package playback

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"io"
)

type mockSQLDriverRows struct {
	ColumnsSet []string
	ValuesSet  [][]driver.Value
	cursor     uint64
}

func newMockSQLDriverRowsFrom(rowsSource driver.Rows) *mockSQLDriverRows {
	defer rowsSource.Close()

	columns := rowsSource.Columns()
	rows := newMockSQLDriverRows()
	rows.ColumnsSet = columns

	count := len(columns)

	for {
		values := make([]driver.Value, count)
		err := rowsSource.Next(values)
		if err != nil {
			break
		}

		rows.AppendValues(values)
	}

	return rows
}

func newMockSQLDriverRows() *mockSQLDriverRows {
	return &mockSQLDriverRows{
		ColumnsSet: []string{},
		ValuesSet:  make([][]driver.Value, 0, 2),
	}
}

func (rows *mockSQLDriverRows) Columns() []string {
	return rows.ColumnsSet
}

func (rows *mockSQLDriverRows) Close() error {
	rows = &mockSQLDriverRows{}
	return nil
}

func (rows *mockSQLDriverRows) Next(dest []driver.Value) error {
	if len(rows.ValuesSet) <= 0 {
		return sql.ErrNoRows
	} else if len(rows.ValuesSet) <= int(rows.cursor) {
		return io.EOF
	}

	copy(dest, rows.ValuesSet[rows.cursor])
	rows.cursor++

	return nil
}

func (rows *mockSQLDriverRows) AppendValues(values []driver.Value) {
	rows.ValuesSet = append(rows.ValuesSet, values)
}

func (rows *mockSQLDriverRows) Marshal() []byte {
	dump, _ := json.Marshal(rows)
	return dump
}

func (rows *mockSQLDriverRows) Unmarshal(data []byte) error {
	return json.Unmarshal(data, rows)
}
