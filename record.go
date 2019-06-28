package playback

import (
	"errors"
	"os"
	"time"

	yaml "gopkg.in/yaml.v2"
)

type RecordKind string

const (
	FileMask = "playback.*.yml"

	KindResult      = RecordKind("result")
	KindHTTP        = RecordKind("http")
	KindHTTPRequest = RecordKind("http_request")
	KindGRPCRequest = RecordKind("grpc_request")
	KindSQLRows     = RecordKind("sql_rows")
	KindSQLResult   = RecordKind("sql_result")

	DefaultKey = ""
)

var ErrPlaybackFailed = errors.New("Playback failed")

type record struct {
	// TODO Obsolete - check
	debounce time.Duration
	basename string // TODO REMOVEME
	file     *os.File
	request  string
	response string

	// TODO New fields
	Kind         RecordKind
	Key          string
	ID           uint64
	RequestMeta  string
	Request      string
	ResponseMeta string
	Response     string
	Err          RecordError
	Panic        interface{}

	cassette *Cassette
}

func (r *record) Record() {
	r.cassette.Add(r)
}

func (r *record) RecordRequest() {
	if r.cassette.SyncMode() == SyncModeEveryChange {
		r.Record()
	}
}

func (r *record) RecordResponse() {
	// FIXME Remove me
	r.Record()
}

func (r *record) setID(id uint64) {
	r.ID = id
}

func yamlMarshalString(value interface{}) string {
	return string(yamlMarshal(value))
}

func yamlMarshal(value interface{}) []byte {
	bytes, _ := yaml.Marshal(value)
	return bytes
}

func (r *record) Playback() error {
	err := r.playback()
	if err != nil {
		return ErrPlaybackFailed
	}

	return nil
}

func (r *record) playback() error {
	record, err := r.cassette.Get(r.Kind, r.Key)
	if err != nil {
		return err
	}

	r.RequestMeta = record.RequestMeta
	r.ResponseMeta = record.ResponseMeta
	r.Response = record.Response
	r.Err = record.Err
	r.Panic = record.Panic

	return nil
}

func (r *record) PanicIfHas() {
	if r.Panic == nil {
		return
	}

	r.cassette.Sync()
	panic(r.Panic)
}
