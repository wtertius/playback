package playback

import (
	"database/sql/driver"
	"encoding/json"
)

type MockSQLDriverResult struct {
	ResultLastInsertId int64
	ErrLastInsertId    error
	ResultRowsAffected int64
	ErrRowsAffected    error
}

func NewMockSQLDriverResultFrom(resultSource driver.Result) *MockSQLDriverResult {
	resultLastInsertId, errLastInsertId := resultSource.LastInsertId()
	resultRowsAffected, errRowsAffected := resultSource.RowsAffected()

	result := &MockSQLDriverResult{
		ResultLastInsertId: resultLastInsertId,
		ErrLastInsertId:    errLastInsertId,
		ResultRowsAffected: resultRowsAffected,
		ErrRowsAffected:    errRowsAffected,
	}

	return result
}

func NewMockSQLDriverResult() *MockSQLDriverResult {
	return &MockSQLDriverResult{}
}

func (result *MockSQLDriverResult) LastInsertId() (int64, error) {
	return result.ResultLastInsertId, result.ErrLastInsertId
}

func (result *MockSQLDriverResult) RowsAffected() (int64, error) {
	return result.ResultRowsAffected, result.ErrRowsAffected
}

func (result *MockSQLDriverResult) Marshal() []byte {
	dump, _ := json.Marshal(result)
	return dump
}

func (result *MockSQLDriverResult) Unmarshal(data []byte) error {
	return json.Unmarshal(data, result)
}
