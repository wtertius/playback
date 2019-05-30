package playback

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"

	yaml "gopkg.in/yaml.v2"
)

var errCassetteGetFailed = errors.New("Cassette get failed")
var errCassetteLocked = errors.New("Cassette locked")

type track struct {
	cursor  int
	records []*record
}

type Cassette struct {
	ID string

	writer     Writer
	playback   *Playback
	tracks     map[RecordKind]map[string]*track
	err        error
	recID      uint64
	recordByID map[uint64]*record
	locked     bool
	mode       Mode
	syncMode   SyncMode
}

func newCassette(p *Playback) *Cassette {
	c := &Cassette{playback: p}
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
		// TODO mutex.Lock
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

func (c *Cassette) Mode() Mode {
	// TODO mutex.RLock
	return c.mode
}

func (c *Cassette) SetMode(mode Mode) *Cassette {
	// TODO Lock
	c.mode = mode

	return c
}

func (c *Cassette) WithFile() (*Cassette, error) {
	var err error
	c.writer, err = c.newFileForCassette()
	return c, err
}

func (c *Cassette) newFileForCassette() (*file, error) {
	f, err := ioutil.TempFile("", c.playback.fileMask)
	return &file{f}, err
}

func (c *Cassette) SetSyncMode(syncMode SyncMode) {
	c.syncMode = syncMode
}

func (c *Cassette) Rewind() {
	c.err = nil

	c.recordByID = make(map[uint64]*record, 10)

	for _, kindTracks := range c.tracks {
		for _, keyTrack := range kindTracks {
			keyTrack.cursor = 0
		}
	}
}
func (c *Cassette) reset() {
	c.recID = 0
	c.err = nil
	c.recordByID = make(map[uint64]*record, 10)
	c.tracks = make(map[RecordKind]map[string]*track, 5)

	for _, kindTracks := range c.tracks {
		for _, keyTrack := range kindTracks {
			keyTrack.cursor = 0
		}
	}
}

func (c *Cassette) Lock() {
	// TODO mu.Lock
	c.locked = true

	if c.writer != nil {
		c.Sync()
	}
}

func (c *Cassette) Unlock() {
	// TODO mu.Lock
	c.locked = false
}

func (c *Cassette) Error() error {
	return c.err
}

func (c *Cassette) IsPlaybackSucceeded() bool {
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
		err = c.Sync()
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Cassette) Sync() error {
	return c.writer.Sync()
}

func (c *Cassette) Finalize() error {
	c.Lock()

	if c.writer != nil {
		return c.writer.Close()
	}
	return nil
}

func (c *Cassette) PathType() PathType {
	if c.writer == nil {
		return PathTypeNil
	}

	return c.writer.Type()
}

func (c *Cassette) PathName() string {
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
	// TODO mutex.Lock ?
	c.recID++
	return c.recID
}

func (c *Cassette) GRPCRequest(req interface{}) error {
	rec, err := c.GetLast(KindGRPCRequest, DefaultKey)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal([]byte(rec.RequestDump), req)
	if err != nil {
		return err
	}

	return err
}

func (c *Cassette) SetGRPCRequest(req interface{}) {
	rec := &record{
		Kind:        KindGRPCRequest,
		Key:         DefaultKey,
		RequestDump: yamlMarshalString(&req),
		Request:     reflect.ValueOf(req).Type().String(),
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

	req, err := httpReadRequest(rec.RequestDump)
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

func (c *Cassette) AddResultRecord(key string, value interface{}, panicObject interface{}) {
	recorder := newResultRecorder(c, key, value, panicObject)
	recorder.record()
}

func (c *Cassette) HTTPResponse(req *http.Request) (*http.Response, error) {
	rec, err := c.GetLast(KindHTTPRequest, "")
	if err != nil {
		return nil, err
	}

	return httpReadResponse(rec.Response, req)
}

func (c *Cassette) Get(kind RecordKind, key string) (rec *record, err error) {
	// TODO mutex.RLock
	if c.tracks[kind] == nil || c.tracks[kind][key] == nil {
		c.err = errCassetteGetFailed
		return nil, errCassetteGetFailed
	}

	track := c.tracks[kind][key]
	if len(track.records) <= track.cursor {
		c.err = errCassetteGetFailed
		return nil, errCassetteGetFailed
	}

	rec = track.records[track.cursor]
	track.cursor++

	return rec, nil
}

func (c *Cassette) GetLast(kind RecordKind, key string) (rec *record, err error) {
	// TODO mutex.RLock
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
	// TODO mutex.Lock
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
		c.tracks[rec.Kind] = make(map[string]*track, 5)
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

func (c *Cassette) MarshalToYAML() []byte {
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

	switch c.mode {
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
