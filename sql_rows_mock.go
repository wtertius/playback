package playback

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"io"
	"reflect"
	"time"
)

type MockSQLDriverRows struct {
	ColumnSet   []string
	ColumnTypes []string
	ValueSet    [][]driver.Value
	cursor      uint64
}

func NewMockSQLDriverRowsFrom(rowsSource driver.Rows) *MockSQLDriverRows {
	defer rowsSource.Close()

	columns := rowsSource.Columns()
	rows := NewMockSQLDriverRows()
	rows.ColumnSet = columns

	if rows, ok := rowsSource.(*MockSQLDriverRows); ok {
		rows.defineColumnTypes()
		return rows
	}

	count := len(columns)
	for {
		values := make([]driver.Value, count)
		err := rowsSource.Next(values)
		if err != nil {
			break
		}

		rows.AppendValues(values)
	}

	rows.defineColumnTypes()

	return rows
}

func NewMockSQLDriverRows() *MockSQLDriverRows {
	return &MockSQLDriverRows{
		ColumnSet: []string{},
		ValueSet:  make([][]driver.Value, 0, 2),
	}
}

func (rows *MockSQLDriverRows) Columns() []string {
	return rows.ColumnSet
}

func (rows *MockSQLDriverRows) Close() error {
	rows = &MockSQLDriverRows{}
	return nil
}

func (rows *MockSQLDriverRows) Next(dest []driver.Value) error {
	if len(rows.ValueSet) <= 0 {
		return sql.ErrNoRows
	} else if len(rows.ValueSet) <= int(rows.cursor) {
		return io.EOF
	}

	copy(dest, rows.ValueSet[rows.cursor])
	rows.cursor++

	return nil
}

func (rows *MockSQLDriverRows) AppendValues(values []driver.Value) {
	rows.ValueSet = append(rows.ValueSet, values)
}

func (rows *MockSQLDriverRows) defineColumnTypes() {
	if len(rows.ColumnTypes) > 0 {
		return
	}

	rows.ColumnTypes = make([]string, len(rows.ColumnSet))
	toFill := make(map[int]bool, len(rows.ColumnSet))
	for i := range rows.ColumnSet {
		toFill[i] = true
	}

	for _, row := range rows.ValueSet {
		for i, value := range row {
			if !toFill[i] || value == nil {
				continue
			}

			kind := reflect.TypeOf(value).Kind()
			if kind == reflect.Slice {
				rows.ColumnTypes[i] = "[]" + reflect.TypeOf(value).Elem().Kind().String()
			} else if reflect.TypeOf(value).String() == "time.Time" {
				rows.ColumnTypes[i] = "time.Time"
			} else {
				rows.ColumnTypes[i] = kind.String()
			}

			delete(toFill, i)
			if len(toFill) == 0 {
				return
			}
		}
	}

	for i := range toFill {
		rows.ColumnTypes[i] = ""
	}
}

func (rows MockSQLDriverRows) Marshal() []byte {
	rows.prepareValueTypes()
	dump, _ := json.Marshal(rows)
	rows.restoreValueTypes()

	return dump
}

func (rows MockSQLDriverRows) prepareValueTypes() {
	for _, row := range rows.ValueSet {
		for i, typ := range rows.ColumnTypes {
			if typ == "" || row[i] == nil {
				continue
			}

			typeOf := reflect.TypeOf(row[i])
			if typeOf.Kind() == reflect.Slice {
				if typeOf.Elem().Kind() == reflect.Uint8 {
					switch typ {
					case "[]uint8", "[]byte":
						row[i] = string(row[i].([]uint8))
					}
				}
			}
		}
	}
}

func (rows *MockSQLDriverRows) Unmarshal(data []byte) error {
	err := json.Unmarshal(data, rows)
	if err != nil {
		return err
	}

	return rows.restoreValueTypes()
}

func (rows *MockSQLDriverRows) restoreValueTypes() error {
	for _, row := range rows.ValueSet {
		for i, typ := range rows.ColumnTypes {
			if typ == "" || row[i] == nil {
				continue
			}

			typeOf := reflect.TypeOf(row[i])
			if typeOf.String() == typ {
				continue
			}

			switch typeOf.Kind() {
			case reflect.Float64:
				switch typ {
				case "int", "int64":
					row[i] = int64(row[i].(float64))
				}

			case reflect.String:
				value := row[i].(string)
				switch typ {
				case "[]uint8", "[]byte":
					row[i] = []uint8(value)
				case "time.Time":
					row[i], _ = time.Parse(time.RFC3339Nano, value)
				}
			}
		}
	}

	return nil
}
