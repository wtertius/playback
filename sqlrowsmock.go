package playback

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"io"
)

type MockSQLDriverRows struct {
	RColumns  []string
	ValuesSet [][]driver.Value
	cursor    uint64
}

func NewMockSQLDriverRowsFrom(rowsSource driver.Rows) *MockSQLDriverRows {
	defer rowsSource.Close()

	columns := rowsSource.Columns()
	rows := NewMockSQLDriverRows()
	rows.RColumns = columns

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

func NewMockSQLDriverRows() *MockSQLDriverRows {
	return &MockSQLDriverRows{
		RColumns:  []string{},
		ValuesSet: make([][]driver.Value, 0, 2),
	}
}

func (rows *MockSQLDriverRows) Columns() []string {
	return rows.RColumns
}

func (rows *MockSQLDriverRows) Close() error {
	rows = &MockSQLDriverRows{}
	return nil
}

func (rows *MockSQLDriverRows) Next(dest []driver.Value) error {
	if len(rows.ValuesSet) <= 0 {
		return sql.ErrNoRows
	} else if len(rows.ValuesSet) <= int(rows.cursor) {
		return io.EOF
	}

	copy(dest, rows.ValuesSet[rows.cursor])
	rows.cursor++

	return nil
}

func (rows *MockSQLDriverRows) AppendValues(values []driver.Value) {
	rows.ValuesSet = append(rows.ValuesSet, values)
}

func (rows *MockSQLDriverRows) Marshal() []byte {
	dump, _ := json.Marshal(rows)
	return dump
}

func (rows *MockSQLDriverRows) Unmarshal(data []byte) error {
	return json.Unmarshal(data, rows)
}
