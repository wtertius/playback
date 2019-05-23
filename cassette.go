package playback

import (
	"errors"
	"io"
	"io/ioutil"
	"net/http"

	yaml "gopkg.in/yaml.v2"
)

var errCassetteGetFailed = errors.New("Cassette get failed")
var errCassetteLocked = errors.New("Cassette locked")

type track struct {
	cursor  int
	records []*record
}

type Cassette struct {
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
	c.reset()
	c.mode = p.Mode()

	return c
}

func newCassetteFromFile(p *Playback, filename string) (*Cassette, error) {
	c := &Cassette{playback: p}
	c.writer = newNilNamed(PathTypeFile, filename)

	dump, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	if len(dump) == 0 {
		return nil, ErrPlaybackFailed
	}

	var records []*record
	err = yaml.Unmarshal(dump, &records)
	if err != nil {
		return nil, err
	}

	c.reset()

	for _, rec := range records {
		// TODO mutex.Lock
		c.add(rec)
	}

	c.mode = ModePlayback

	return c, nil
}

func (c *Cassette) Result(key string, value interface{}) interface{} {
	recorder := newResultRecorder(c, key, value)

	c.Run(recorder)

	return recorder.value
}

func (c *Cassette) Mode() Mode {
	// TODO mutex.RLock
	return c.mode
}

func (c *Cassette) SetMode(mode Mode) {
	// TODO Lock
	c.mode = mode
}

func (c *Cassette) WithFile() (*Cassette, error) {
	var err error
	c.writer, err = c.playback.newFileForCassette()
	return c, err
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
	if c.mode != ModePlayback || c.err != nil {
		return false
	}

	for kind, kindTracks := range c.tracks {
		if kind == KindHTTPRequest {
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

func (c *Cassette) HTTPRequest() (*http.Request, error) {
	rec, err := c.GetLast(KindHTTPRequest, "")
	if err != nil {
		return nil, err
	}

	req, err := httpReadRequest(rec.RequestDump)
	return req, err
}

func (c *Cassette) SetHTTPRequest(req *http.Request) {
	rec := c.getHTTPRecord(req)
	c.Add(rec)
}

func (c *Cassette) getHTTPRecord(req *http.Request) *record {
	rec := newHTTPRecorder(nil, req).newRecord(req)
	rec.Kind = KindHTTPRequest
	rec.Key = ""

	return rec
}

func (c *Cassette) SetHTTPResponse(req *http.Request, res *http.Response) {
	rec, err := c.GetLast(KindHTTPRequest, "")
	if err != nil {
		rec = c.getHTTPRecord(req)
	}

	(&httpRecorder{}).RecordResponse(rec, res, nil)
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
	marshalled := yamlMarshal([]*record{rec})
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

func (c *Cassette) Run(recorder Recorder) error {
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
