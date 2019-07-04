package playback

import (
	"database/sql/driver"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMockSQLDriverRows(t *testing.T) {
	date, _ := time.Parse(time.RFC3339, "2019-07-09T16:00:00+03:00")

	t.Run("marshal & unmarshal", func(t *testing.T) {
		rowsHad := &MockSQLDriverRows{
			ColumnSet:   []string{"id", "float_[]uint8", "string", "bytes", "float", "bool", "time", "nil", "typedNil"},
			ColumnTypes: []string{"int64", "[]uint8", "string", "[]byte", "float64", "bool", "time.Time", "", "int64"},
			ValueSet:    [][]driver.Value{{int64(10), []uint8("750.0000"), "hi", []byte("hello"), 7.00, true, date, nil, nil}},
		}

		dump := rowsHad.Marshal()
		assert.NotEmpty(t, dump)

		rowsGot := NewMockSQLDriverRows()
		err := rowsGot.Unmarshal(dump)
		assert.Nil(t, err)

		assert.Equal(t, rowsHad, rowsGot)
	})

	t.Run("defineColumnTypes", func(t *testing.T) {
		t.Run("in first row", func(t *testing.T) {
			rows := &MockSQLDriverRows{
				ColumnSet: []string{"id", "float_[]uint8", "string", "bytes", "float", "bool", "time"},
				ValueSet: [][]driver.Value{
					{int64(10), []uint8("750.0000"), "hi", []byte("hello"), 7.00, true, date},
					{nil, nil, nil, nil, nil, nil, nil, nil},
				},
			}

			rows.defineColumnTypes()

			columnTypesHad := []string{"int64", "[]uint8", "string", "[]uint8", "float64", "bool", "time.Time"}
			assert.Equal(t, columnTypesHad, rows.ColumnTypes)
		})
		t.Run("in all rows", func(t *testing.T) {
			rows := &MockSQLDriverRows{
				ColumnSet: []string{"id", "float_[]uint8", "string", "bytes", "float", "bool", "time", "nil"},
				ValueSet: [][]driver.Value{
					{int64(10), nil, nil, nil, nil, nil, nil, nil},
					{nil, []uint8("750.0000"), nil, nil, nil, nil, nil, nil},
					{nil, nil, "hi", nil, nil, nil, nil, nil},
					{nil, nil, nil, []byte("hello"), nil, nil, nil, nil},
					{nil, nil, nil, nil, 7.00, nil, nil, nil},
					{nil, nil, nil, nil, nil, true, nil, nil},
					{nil, nil, nil, nil, nil, nil, date, nil},
				},
			}

			rows.defineColumnTypes()

			columnTypesHad := []string{"int64", "[]uint8", "string", "[]uint8", "float64", "bool", "time.Time", ""}
			assert.Equal(t, columnTypesHad, rows.ColumnTypes)
		})
	})
}
