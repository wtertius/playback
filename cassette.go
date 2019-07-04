package playback

import (
	"bytes"
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"sync"

	"github.com/sergi/go-diff/diffmatchpatch"
	yaml "gopkg.in/yaml.v2"
)

var errCassetteGetFailed = errors.New("Cassette get failed")
var errCassetteLocked = errors.New("Cassette locked")

type track struct {
	cursor  int
	records []*record
}

type trackMap map[string]*track

type Cassette struct {
	ID string

	writer     Writer
	playback   *Playback
	tracks     map[RecordKind]trackMap
	err        error
	recID      uint64
	recordByID map[uint64]*record
	locked     bool
	mode       Mode
	syncMode   SyncMode
	debug      bool
	logger     Logger
	mu         sync.RWMutex
}

func newCassette(p *Playback) *Cassette {
	c := &Cassette{
		playback: p,
		logger:   p.getLogger(),
		debug:    p.Debug(),
	}
	c.ID = p.generateID()
	c.reset()
	c.mode = p.Mode()

	p.Add(c)

	return c
}

func newCassetteFromFile(p *Playback, filename string) (*Cassette, error) {
	dump, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	c, err := newCassetteFromYAML(p, dump)
	if err != nil {
		return c, err
	}

	c.writer = newNilNamed(PathTypeFile, filename)

	return c, nil
}

func newCassetteFromYAML(p *Playback, dump []byte) (*Cassette, error) {
	if len(dump) == 0 {
		return nil, ErrPlaybackFailed
	}

	var records []*record
	err := yaml.Unmarshal(dump, &records)
	if err != nil {
		return nil, err
	}

	c := newCassette(p)

	for _, rec := range records {
		c.add(rec)
	}

	c.mode = ModePlayback

	return c, nil
}

func (c *Cassette) Result(key string, value interface{}) interface{} {
	recorder := newResultRecorder(c, key, value, nil)

	c.Run(recorder)

	return recorder.value
}

func (c *Cassette) ResultWithError(key string, value interface{}) (interface{}, error) {
	recorder := newResultRecorder(c, key, value, nil)

	c.Run(recorder)

	return recorder.value, recorder.err
}

func (c *Cassette) Mode() Mode {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.mode
}

func (c *Cassette) SetMode(mode Mode) *Cassette {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.mode = mode

	return c
}

func (c *Cassette) Debug() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.debug
}

func (c *Cassette) SetDebug(debug bool) *Cassette {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.debug = debug

	return c
}

func (c *Cassette) SetLogger(logger Logger) *Cassette {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.logger = logger

	return c
}

func (c *Cassette) WithFile() (*Cassette, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var err error
	c.writer, err = c.newFileForCassette()
	return c, err
}

func (c *Cassette) newFileForCassette() (*file, error) {
	f, err := ioutil.TempFile("", c.playback.fileMask)
	return &file{f}, err
}

func (c *Cassette) SyncMode() SyncMode {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.syncMode
}

func (c *Cassette) SetSyncMode(syncMode SyncMode) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.syncMode = syncMode
}

func (c *Cassette) Rewind() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.err = nil

	c.recordByID = make(map[uint64]*record, 10)

	for _, kindTracks := range c.tracks {
		for _, keyTrack := range kindTracks {
			keyTrack.cursor = 0
		}
	}
}
func (c *Cassette) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.recID = 0
	c.err = nil
	c.recordByID = make(map[uint64]*record, 10)
	c.tracks = make(map[RecordKind]trackMap, 5)

	for _, kindTracks := range c.tracks {
		for _, keyTrack := range kindTracks {
			keyTrack.cursor = 0
		}
	}
}

func (c *Cassette) Lock() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lock()
}

func (c *Cassette) lock() {
	c.locked = true

	if c.writer != nil {
		c.sync()
	}
}

func (c *Cassette) Unlock() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.locked = false
}

func (c *Cassette) Error() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.err
}

func (c *Cassette) IsPlaybackSucceeded() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c == nil || c.mode != ModePlayback || c.err != nil {
		return false
	}

	for kind, kindTracks := range c.tracks {
		if kind == KindHTTPRequest || kind == KindGRPCRequest {
			continue
		}

		for _, keyTrack := range kindTracks {
			if keyTrack.cursor != len(keyTrack.records) {
				return false
			}
		}
	}

	return true
}

func (c *Cassette) IsGRPCResponseCorrect(res interface{}) bool {
	resExpected := reflect.New(reflect.TypeOf(res).Elem()).Interface()
	err := c.GRPCResponse(resExpected)
	if err != nil {
		return false
	}

	err = c.GRPCResponse(res)
	if err != nil {
		return false
	}

	return reflect.DeepEqual(resExpected, res)
}

func (c *Cassette) IsHTTPResponseCorrect(res *http.Response) bool {
	req, err := c.HTTPRequest()
	if err != nil {
		return false
	}

	res = httpDeleteHeaders(httpCopyResponse(res, req))

	resExpected, _ := c.HTTPResponse(req)
	resExpected = httpDeleteHeaders(httpCopyResponse(resExpected, req))

	return httpDumpResponse(res) == httpDumpResponse(resExpected)
}

func (c *Cassette) write(content string) error {
	if c.writer == nil || c.writer.ReadOnly() {
		return nil
	}

	_, err := io.WriteString(c.writer, content)
	if err != nil {
		return err
	}

	if c.syncMode == SyncModeEveryChange {
		err = c.sync()
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Cassette) Sync() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.sync()
}

func (c *Cassette) sync() error {
	return c.writer.Sync()
}

func (c *Cassette) Finalize() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lock()

	if c.writer != nil {
		return c.writer.Close()
	}
	return nil
}

func (c *Cassette) PathType() PathType {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.writer == nil {
		return PathTypeNil
	}

	return c.writer.Type()
}

func (c *Cassette) PathName() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.writer == nil {
		return ""
	}

	return c.writer.Name()
}

func (c *Cassette) setID(rec *record) {
	if rec.ID == 0 {
		rec.setID(c.nextRecordID())
	}
}

func (c *Cassette) nextRecordID() uint64 {
	c.recID++
	return c.recID
}

func (c *Cassette) GRPCRequest(req interface{}) error {
	rec, err := c.GetLast(KindGRPCRequest, DefaultKey)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal([]byte(rec.Request), req)
	if err != nil {
		return err
	}

	return err
}

func (c *Cassette) SetGRPCRequest(req interface{}) {
	rec := &record{
		Kind:        KindGRPCRequest,
		Key:         DefaultKey,
		Request:     yamlMarshalString(&req),
		RequestMeta: reflect.ValueOf(req).Type().String(),
	}
	c.Add(rec)
}

func (c *Cassette) GRPCResponse(resp interface{}) error {
	rec, err := c.GetLast(KindGRPCRequest, DefaultKey)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal([]byte(rec.Response), resp)
	if err != nil {
		return err
	}

	return err
}

func (c *Cassette) SetGRPCResponse(resp interface{}) error {
	rec, err := c.GetLast(KindGRPCRequest, DefaultKey)
	if err != nil {
		return err
	}

	rec.ResponseMeta = reflect.ValueOf(resp).Type().String()
	rec.Response = yamlMarshalString(&resp)

	return c.Add(rec)
}

func (c *Cassette) HTTPRequest() (*http.Request, error) {
	rec, err := c.GetLast(KindHTTPRequest, DefaultKey)
	if err != nil {
		return nil, err
	}

	req, err := httpReadRequest(rec.Request)
	return req, err
}

func (c *Cassette) SetHTTPRequest(req *http.Request) {
	rec := c.buildHTTPRecord(req)
	c.Add(rec)
}

func (c *Cassette) buildHTTPRecord(req *http.Request) *record {
	rec := newHTTPRecorder(nil, req).newRecord(req)
	rec.Kind = KindHTTPRequest
	rec.Key = ""
	rec.cassette = c

	return rec
}

func (c *Cassette) SetHTTPResponse(req *http.Request, res *http.Response) {
	rec, err := c.GetLast(KindHTTPRequest, "")
	if err != nil {
		rec = c.buildHTTPRecord(req)
	}

	(&HTTPRecorder{rec: rec}).RecordResponse(res, nil)
}

func (c *Cassette) AddHTTPRecord(req *http.Request, res *http.Response, err error) *HTTPRecorder {
	recorder := &HTTPRecorder{cassette: c}
	recorder.newRecord(req)
	if res == nil && err == nil {
		recorder.rec.RecordRequest()
		return recorder
	}

	recorder.RecordResponse(res, nil)
	return recorder
}

func (c *Cassette) AddResultRecord(key string, typString string, value interface{}, err error, panicObject interface{}) {
	recorder := newResultRecorder(c, key, value, panicObject)
	recorder.typString = typString
	recorder.record()
}

func (c *Cassette) AddSQLRows(query string, rows driver.Rows, err error, options ...SQLRowsRecorderOption) *SQLRowsRecorder {
	recorder := &SQLRowsRecorder{
		cassette: c,
		query:    query,
	}

	recorder.ApplyOptions(options...)

	recorder.newRecord(context.Background(), query)
	if rows == nil && err == nil {
		recorder.rec.RecordRequest()
		return recorder
	}

	recorder.RecordResponse(rows, err)
	return recorder
}

func (c *Cassette) HTTPResponse(req *http.Request) (*http.Response, error) {
	rec, err := c.GetLast(KindHTTPRequest, "")
	if err != nil {
		return nil, err
	}

	return httpReadResponse(rec.Response, req)
}

func (c *Cassette) Get(kind RecordKind, key string) (*record, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.get(kind, key)
}

func (c *Cassette) get(kind RecordKind, key string) (*record, error) {
	track, err := c.getTrack(kind, key)
	if err != nil {
		return nil, err
	}

	rec := track.records[track.cursor]
	track.cursor++

	return rec, nil
}

func (c *Cassette) getTrack(kind RecordKind, key string) (*track, error) {
	if c.tracks[kind] == nil || c.tracks[kind][key] == nil {
		c.err = errCassetteGetFailed
		return nil, errCassetteGetFailed
	}

	track := c.tracks[kind][key]
	if len(track.records) <= track.cursor {
		c.err = errCassetteGetFailed
		return nil, errCassetteGetFailed
	}

	return track, nil
}

func (c *Cassette) getByPrefix(kind RecordKind, prefix string) (rec *record, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.tracks[kind] == nil {
		c.err = errCassetteGetFailed
		return nil, errCassetteGetFailed
	}

	for key := range c.tracks[kind] {
		if strings.HasPrefix(key, prefix) {
			track, err := c.getTrack(kind, key)
			if err != nil {
				return nil, err
			}

			rec := track.records[track.cursor]
			return rec, nil
		}
	}

	return nil, errCassetteGetFailed
}

func (c *Cassette) getByKind(kind RecordKind) ([]*record, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.tracks[kind] == nil {
		c.err = errCassetteGetFailed
		return nil, errCassetteGetFailed
	}

	records := make([]*record, 0, len(c.tracks[kind])*2)
	for _, track := range c.tracks[kind] {
		for _, rec := range track.records {
			records = append(records, rec)
		}
	}

	return records, nil
}

func (c *Cassette) Keys() map[RecordKind]map[string]struct{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make(map[RecordKind]map[string]struct{}, len(c.tracks))

	for kind, kindTracks := range c.tracks {
		for key := range kindTracks {
			if _, ok := keys[kind]; !ok {
				keys[kind] = make(map[string]struct{}, len(kindTracks))
			}

			keys[kind][key] = struct{}{}
		}
	}

	return keys
}

func (c *Cassette) GetLast(kind RecordKind, key string) (rec *record, err error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.tracks[kind] == nil || c.tracks[kind][key] == nil {
		c.err = errCassetteGetFailed
		return nil, errCassetteGetFailed
	}

	track := c.tracks[kind][key]
	if len(track.records) == 0 {
		c.err = errCassetteGetFailed
		return nil, errCassetteGetFailed
	}

	rec = track.records[len(track.records)-1]

	return rec, nil
}

func (c *Cassette) Add(rec *record) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.locked {
		c.err = errCassetteLocked
		return errCassetteLocked
	}

	c.add(rec)
	marshalled := yamlMarshalString([]*record{rec})
	return c.write(marshalled)
}

func (c *Cassette) add(rec *record) {
	c.setID(rec)
	if c.recordByID[rec.ID] != nil {
		*(c.recordByID[rec.ID]) = *rec
		return
	}

	c.recordByID[rec.ID] = rec

	if c.tracks[rec.Kind] == nil {
		c.tracks[rec.Kind] = make(trackMap, 5)
	}
	if c.tracks[rec.Kind][rec.Key] == nil {
		c.tracks[rec.Kind][rec.Key] = &track{
			cursor:  0,
			records: make([]*record, 0, 2),
		}
	}
	track := c.tracks[rec.Kind][rec.Key]
	track.records = append(track.records, rec)
}

func (c *Cassette) debugRecordMatch(rec *record, kind RecordKind, prefix string) {
	if !c.Debug() {
		return
	}

	recMatched, e := c.getByPrefix(kind, prefix)
	if e == nil {
		c.diffRecords(fmt.Sprintf("Can't find match by key '%s'.", rec.Key), rec, recMatched)
		return
	}

	c.logger.Debugf("Can't find match by key '%s' or prefix '%s'.\n", rec.Key, prefix)
	c.logger.Debugf("<<ALL_MATCHES\n")

	records, e := c.getByKind(kind)
	for i, recHTTP := range records {
		c.diffRecords(fmt.Sprintf("Listing all HTTP options. Option %d:", i), rec, recHTTP)
	}

	c.logger.Debugf("ALL_MATCHES\n")
}

func (c *Cassette) diffRecords(phrase string, recordOriginal, recordMatched *record) {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(recordOriginal.Request, recordMatched.Request, false)

	c.logger.Debugf("%s\n\n"+
		"Original: <<END\n%s\nEND\n\n"+
		"Nearest match <<END:\n%s\nEND\n\n"+
		"Difference: <<END\n%s\nEND\n\n",
		phrase,
		recordOriginal.Request,
		recordMatched.Request,
		dmp.DiffPrettyText(diffs),
	)
}

func (c *Cassette) MarshalToYAML() []byte {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var buf bytes.Buffer
	for _, kindTracks := range c.tracks {
		for _, keyTrack := range kindTracks {
			buf.Write(yamlMarshal(keyTrack.records))
		}
	}

	return buf.Bytes()
}

func (c *Cassette) Run(recorder Recorder) error {
	if c == nil {
		return recorder.Call()
	}

	switch c.Mode() {
	case ModeOff:
		return recorder.Call()
	case ModePlayback:
		return recorder.Playback()

	case ModePlaybackOrRecord:
		err := recorder.Playback()
		if err == ErrPlaybackFailed {
			return recorder.Record()
		}
		return err

	case ModePlaybackSuccessOrRecord:
		err := recorder.Playback()
		if err != nil {
			return recorder.Record()
		}
		return err

	case ModeRecord:
		return recorder.Record()

	}

	return recorder.Call()
}
